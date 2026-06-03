package main

import (
  "context"
  "encoding/json"
  "errors"
  "fmt"
  "html/template"
  "log"
  "net/http"
  "os"
  "os/exec"
  "path/filepath"
  "sort"
  "strconv"
  "strings"
  "sync"
  "time"

  "github.com/redis/go-redis/v9"
)

type config struct {
  RedisAddr     string
  ComposeDir    string
  LabTTL        time.Duration
  MaxSlots      int
  BasePort      int
  PortStep      int
  BackendHost   string
  PublicBaseURL string
  NginxContainer string
  NginxLabsPath string
  LabsPageDir   string
}

type labInfo struct {
  Slot      int       `json:"slot"`
  Name      string    `json:"name"`
  Prefix    string    `json:"prefix"`
  BasePort  int       `json:"basePort"`
  ExpiresAt time.Time `json:"expiresAt"`
}

type labResponse struct {
  ID        string            `json:"id"`
  Name      string            `json:"name"`
  Slot      int               `json:"slot"`
  BasePort  int               `json:"basePort"`
  ExpiresAt string            `json:"expiresAt"`
  Paths     map[string]string `json:"paths"`
  URLs      map[string]string `json:"urls"`
  PagePath  string            `json:"pagePath"`
  PageURL   string            `json:"pageURL"`
}

var (
  cfg        config
  rdb        *redis.Client
  nginxMu    sync.Mutex
)

func main() {
  cfg = loadConfig()
  if cfg.ComposeDir == "" {
    log.Fatal("COMPOSE_DIR is required")
  }

  rdb = redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
  if err := rdb.Ping(context.Background()).Err(); err != nil {
    log.Fatalf("redis ping failed: %v", err)
  }

  enableKeyspaceNotifications(context.Background())

  go cleanupLoop()

  mux := http.NewServeMux()
  mux.HandleFunc("/api/health", healthHandler)
  mux.HandleFunc("/api/labs", labsHandler)

  addr := ":9002"
  log.Printf("provisioner listening on %s", addr)
  if err := http.ListenAndServe(addr, mux); err != nil {
    log.Fatal(err)
  }
}

func loadConfig() config {
  ttlSeconds := getEnvInt("LAB_TTL_SECONDS", 7200)
  maxSlots := getEnvInt("MAX_SLOTS", 15)
  basePort := getEnvInt("BASE_PORT", 7681)
  portStep := getEnvInt("PORT_STEP", 10)

  return config{
    RedisAddr:     getEnv("REDIS_ADDR", "127.0.0.1:6379"),
    ComposeDir:    os.Getenv("COMPOSE_DIR"),
    LabTTL:        time.Duration(ttlSeconds) * time.Second,
    MaxSlots:      maxSlots,
    BasePort:      basePort,
    PortStep:      portStep,
    BackendHost:   getEnv("BACKEND_HOST", "host.docker.internal"),
    PublicBaseURL: strings.TrimRight(getEnv("PUBLIC_BASE_URL", "https://netlab.ioda.com.br"), "/"),
    NginxContainer: getEnv("NGINX_CONTAINER", "bgp_lab1_nginx"),
    NginxLabsPath: getEnv("NGINX_LABS_PATH", "/data/nginx/labs/labs.conf"),
    LabsPageDir:   getEnv("LABS_PAGE_DIR", "/data/onboarding/labs"),
  }
}

func getEnv(key, fallback string) string {
  value := strings.TrimSpace(os.Getenv(key))
  if value == "" {
    return fallback
  }
  return value
}

func getEnvInt(key string, fallback int) int {
  value := strings.TrimSpace(os.Getenv(key))
  if value == "" {
    return fallback
  }
  parsed, err := strconv.Atoi(value)
  if err != nil {
    return fallback
  }
  return parsed
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
  w.WriteHeader(http.StatusOK)
  _, _ = w.Write([]byte("ok"))
}

type labRequest struct {
  Name string `json:"name"`
}

func labsHandler(w http.ResponseWriter, r *http.Request) {
  if r.Method != http.MethodPost {
    http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    return
  }

  var req labRequest
  if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
    http.Error(w, "invalid json", http.StatusBadRequest)
    return
  }

  name := normalizeName(req.Name)
  if name == "" {
    http.Error(w, "name is required", http.StatusBadRequest)
    return
  }

  ctx := r.Context()
  info, err := getOrCreateLab(ctx, name)
  if err != nil {
    if errors.Is(err, errNoSlots) {
      http.Error(w, "no slots available", http.StatusServiceUnavailable)
      return
    }
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
  }

  resp := buildResponse(info)
  w.Header().Set("Content-Type", "application/json")
  _ = json.NewEncoder(w).Encode(resp)
}

var errNoSlots = errors.New("no slots")

func getOrCreateLab(ctx context.Context, name string) (labInfo, error) {
  if slot, err := getSlotByName(ctx, name); err == nil {
    info, err := getLabData(ctx, slot)
    if err == nil {
      return info, nil
    }
  }

  expiresAt := time.Now().Add(cfg.LabTTL)
  for slot := 1; slot <= cfg.MaxSlots; slot++ {
    info := labInfo{
      Slot:      slot,
      Name:      name,
      Prefix:    fmt.Sprintf("aluno%02d", slot),
      BasePort:  cfg.BasePort + (slot-1)*cfg.PortStep,
      ExpiresAt: expiresAt,
    }

    if ok, err := tryReserveSlot(ctx, info); err != nil {
      return labInfo{}, err
    } else if !ok {
      continue
    }

    if err := startLab(ctx, info); err != nil {
      releaseSlot(ctx, info)
      return labInfo{}, err
    }

    if err := writeLabPage(info); err != nil {
      log.Printf("write lab page failed: %v", err)
    }

    if err := refreshNginxRoutes(ctx); err != nil {
      log.Printf("nginx refresh failed: %v", err)
    }

    return info, nil
  }

  return labInfo{}, errNoSlots
}

func normalizeName(name string) string {
  trimmed := strings.TrimSpace(strings.ToLower(name))
  trimmed = strings.ReplaceAll(trimmed, "  ", " ")
  return trimmed
}

func getSlotByName(ctx context.Context, name string) (int, error) {
  value, err := rdb.Get(ctx, fmt.Sprintf("lab:name:%s", name)).Result()
  if err != nil {
    return 0, err
  }
  slot, err := strconv.Atoi(value)
  if err != nil {
    return 0, err
  }
  return slot, nil
}

func tryReserveSlot(ctx context.Context, info labInfo) (bool, error) {
  slotKey := fmt.Sprintf("lab:slot:%d", info.Slot)
  dataKey := fmt.Sprintf("lab:data:%d", info.Slot)

  payload, _ := json.Marshal(info)
  ok, err := rdb.SetNX(ctx, slotKey, payload, cfg.LabTTL).Result()
  if err != nil || !ok {
    return ok, err
  }

  if err := rdb.Set(ctx, fmt.Sprintf("lab:name:%s", info.Name), info.Slot, cfg.LabTTL).Err(); err != nil {
    rdb.Del(ctx, slotKey)
    return false, err
  }

  if err := rdb.Set(ctx, dataKey, payload, 0).Err(); err != nil {
    rdb.Del(ctx, slotKey)
    rdb.Del(ctx, fmt.Sprintf("lab:name:%s", info.Name))
    return false, err
  }

  if err := rdb.ZAdd(ctx, "labs:active", redis.Z{Score: float64(info.ExpiresAt.Unix()), Member: info.Slot}).Err(); err != nil {
    return false, err
  }

  return true, nil
}

func getLabData(ctx context.Context, slot int) (labInfo, error) {
  data, err := rdb.Get(ctx, fmt.Sprintf("lab:data:%d", slot)).Result()
  if err != nil {
    return labInfo{}, err
  }
  var info labInfo
  if err := json.Unmarshal([]byte(data), &info); err != nil {
    return labInfo{}, err
  }
  return info, nil
}

func releaseSlot(ctx context.Context, info labInfo) {
  rdb.Del(ctx,
    fmt.Sprintf("lab:slot:%d", info.Slot),
    fmt.Sprintf("lab:data:%d", info.Slot),
    fmt.Sprintf("lab:name:%s", info.Name),
  )
  rdb.ZRem(ctx, "labs:active", info.Slot)
}

func startLab(ctx context.Context, info labInfo) error {
  env := os.Environ()
  env = append(env,
    fmt.Sprintf("LAB_PREFIX=%s", info.Prefix),
    fmt.Sprintf("R1_PORT=%d", info.BasePort),
    fmt.Sprintf("R2_PORT=%d", info.BasePort+1),
    fmt.Sprintf("R3_PORT=%d", info.BasePort+2),
    fmt.Sprintf("R4_PORT=%d", info.BasePort+3),
  )

  cmd := exec.CommandContext(ctx, "docker", "compose", "--project-name", info.Prefix, "up", "-d")
  cmd.Dir = cfg.ComposeDir
  cmd.Env = env
  output, err := cmd.CombinedOutput()
  if err != nil {
    return fmt.Errorf("compose up failed: %w: %s", err, strings.TrimSpace(string(output)))
  }
  return nil
}

func stopLab(ctx context.Context, info labInfo) error {
  cmd := exec.CommandContext(ctx, "docker", "compose", "--project-name", info.Prefix, "down")
  cmd.Dir = cfg.ComposeDir
  output, err := cmd.CombinedOutput()
  if err != nil {
    return fmt.Errorf("compose down failed: %w: %s", err, strings.TrimSpace(string(output)))
  }
  return nil
}

func buildResponse(info labInfo) labResponse {
  paths := map[string]string{
    "r1": fmt.Sprintf("/%s/r1/", info.Prefix),
    "r2": fmt.Sprintf("/%s/r2/", info.Prefix),
    "r3": fmt.Sprintf("/%s/r3/", info.Prefix),
    "r4": fmt.Sprintf("/%s/r4/", info.Prefix),
  }

  pagePath := fmt.Sprintf("/%s/", info.Prefix)

  urls := map[string]string{}
  for key, path := range paths {
    urls[key] = cfg.PublicBaseURL + path
  }

  return labResponse{
    ID:        info.Prefix,
    Name:      info.Name,
    Slot:      info.Slot,
    BasePort:  info.BasePort,
    ExpiresAt: info.ExpiresAt.Format(time.RFC3339),
    Paths:     paths,
    URLs:      urls,
    PagePath:  pagePath,
    PageURL:   cfg.PublicBaseURL + pagePath,
  }
}

type labPageData struct {
  Name      string
  Prefix    string
  ExpiresAt string
  BaseURL   string
}

const labPageTemplate = `<!doctype html>
<html lang="pt-br">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Lab {{.Prefix}} - BGP Lab 1</title>
  <style>
    :root {
      --bg: #0b0f1a;
      --card: #161e33;
      --line: #233050;
      --ink: #f5f7ff;
      --muted: #b8c0d6;
      --accent: #ffb000;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: "Space Grotesk", system-ui, -apple-system, sans-serif;
      background: linear-gradient(180deg, #0b0f1a, #12182a);
      color: var(--ink);
    }
    .wrap {
      max-width: 900px;
      margin: 0 auto;
      padding: 40px 22px 60px;
    }
    .card {
      background: var(--card);
      border: 1px solid var(--line);
      border-radius: 16px;
      padding: 18px;
      margin-bottom: 16px;
    }
    h1 { margin: 0 0 6px; font-size: 28px; }
    p { color: var(--muted); }
    .links {
      display: grid;
      gap: 10px;
      margin-top: 14px;
    }
    a {
      display: block;
      padding: 12px 14px;
      border-radius: 12px;
      border: 1px solid var(--line);
      text-decoration: none;
      color: var(--ink);
      background: #0f1528;
    }
    .pill {
      display: inline-flex;
      padding: 4px 10px;
      border-radius: 999px;
      border: 1px solid var(--line);
      color: var(--muted);
      font-size: 12px;
      letter-spacing: 0.03em;
      text-transform: uppercase;
    }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="card">
      <span class="pill">{{.Prefix}}</span>
      <h1>Seu lab esta pronto</h1>
      <p>Aluno: {{.Name}} · Expira em {{.ExpiresAt}}</p>
    </div>
    <div class="card">
      <h2>Entrar nos roteadores</h2>
      <div class="links">
        <a href="{{.BaseURL}}/{{.Prefix}}/r1/" target="_blank" rel="noopener">R1 - AS1</a>
        <a href="{{.BaseURL}}/{{.Prefix}}/r2/" target="_blank" rel="noopener">R2 - AS2</a>
        <a href="{{.BaseURL}}/{{.Prefix}}/r3/" target="_blank" rel="noopener">R3 - AS3</a>
        <a href="{{.BaseURL}}/{{.Prefix}}/r4/" target="_blank" rel="noopener">R4 - AS4</a>
      </div>
    </div>
    <div class="card">
      <h2>O que fazer agora</h2>
      <p>Abra o roteiro completo na aba "Roteiro completo" da pagina inicial. Em cada roteador, valide BGP e execute as etapas de prepend e MED.</p>
    </div>
  </div>
</body>
</html>
`

func writeLabPage(info labInfo) error {
  if cfg.LabsPageDir == "" {
    return nil
  }

  pageDir := filepath.Join(cfg.LabsPageDir, info.Prefix)
  if err := os.MkdirAll(pageDir, 0755); err != nil {
    return fmt.Errorf("create lab page dir: %w", err)
  }

  pagePath := filepath.Join(pageDir, "index.html")
  tmpl, err := template.New("lab-page").Parse(labPageTemplate)
  if err != nil {
    return fmt.Errorf("parse lab page template: %w", err)
  }

  file, err := os.Create(pagePath)
  if err != nil {
    return fmt.Errorf("create lab page: %w", err)
  }
  defer file.Close()

  data := labPageData{
    Name:      info.Name,
    Prefix:    info.Prefix,
    ExpiresAt: info.ExpiresAt.Format(time.RFC3339),
    BaseURL:   cfg.PublicBaseURL,
  }

  if err := tmpl.Execute(file, data); err != nil {
    return fmt.Errorf("render lab page: %w", err)
  }

  return nil
}

func removeLabPage(info labInfo) error {
  if cfg.LabsPageDir == "" {
    return nil
  }
  pageDir := filepath.Join(cfg.LabsPageDir, info.Prefix)
  if err := os.RemoveAll(pageDir); err != nil {
    return fmt.Errorf("remove lab page dir: %w", err)
  }
  return nil
}

func refreshNginxRoutes(ctx context.Context) error {
  nginxMu.Lock()
  defer nginxMu.Unlock()

  infos, err := listActiveLabs(ctx)
  if err != nil {
    return err
  }

  var builder strings.Builder
  for _, info := range infos {
    prefix := info.Prefix
    builder.WriteString(buildLocation(prefix, "r1", info.BasePort))
    builder.WriteString(buildLocation(prefix, "r2", info.BasePort+1))
    builder.WriteString(buildLocation(prefix, "r3", info.BasePort+2))
    builder.WriteString(buildLocation(prefix, "r4", info.BasePort+3))
  }

  if err := os.WriteFile(cfg.NginxLabsPath, []byte(builder.String()), 0644); err != nil {
    return fmt.Errorf("write nginx routes: %w", err)
  }

  if err := reloadNginx(ctx); err != nil {
    return err
  }

  return nil
}

func listActiveLabs(ctx context.Context) ([]labInfo, error) {
  slots, err := rdb.ZRange(ctx, "labs:active", 0, -1).Result()
  if err != nil {
    return nil, err
  }

  infos := make([]labInfo, 0, len(slots))
  for _, slotStr := range slots {
    slot, err := strconv.Atoi(slotStr)
    if err != nil {
      continue
    }
    info, err := getLabData(ctx, slot)
    if err != nil {
      continue
    }
    infos = append(infos, info)
  }

  sort.Slice(infos, func(i, j int) bool { return infos[i].Slot < infos[j].Slot })
  return infos, nil
}

func buildLocation(prefix, router string, port int) string {
  return fmt.Sprintf("\n  location /%s/%s/ {\n    proxy_pass http://%s:%d/;\n    proxy_http_version 1.1;\n    proxy_set_header Upgrade $http_upgrade;\n    proxy_set_header Connection \"upgrade\";\n    proxy_set_header Host $host;\n  }\n", prefix, router, cfg.BackendHost, port)
}

func reloadNginx(ctx context.Context) error {
  cmd := exec.CommandContext(ctx, "docker", "exec", cfg.NginxContainer, "nginx", "-s", "reload")
  output, err := cmd.CombinedOutput()
  if err != nil {
    return fmt.Errorf("nginx reload failed: %w: %s", err, strings.TrimSpace(string(output)))
  }
  return nil
}

func cleanupLoop() {
  ticker := time.NewTicker(1 * time.Minute)
  defer ticker.Stop()

  for range ticker.C {
    ctx := context.Background()
    now := time.Now().Unix()
    expired, err := rdb.ZRangeByScore(ctx, "labs:active", &redis.ZRangeBy{Min: "-inf", Max: fmt.Sprintf("%d", now)}).Result()
    if err != nil {
      log.Printf("cleanup scan failed: %v", err)
      continue
    }

    if len(expired) == 0 {
      continue
    }

    for _, slotStr := range expired {
      slot, err := strconv.Atoi(slotStr)
      if err != nil {
        continue
      }
      info, err := getLabData(ctx, slot)
      if err != nil {
        continue
      }
      if err := stopLab(ctx, info); err != nil {
        log.Printf("stop lab %s failed: %v", info.Prefix, err)
      }
      if err := removeLabPage(info); err != nil {
        log.Printf("remove lab page %s failed: %v", info.Prefix, err)
      }
      releaseSlot(ctx, info)
    }

    if err := refreshNginxRoutes(ctx); err != nil {
      log.Printf("nginx refresh after cleanup failed: %v", err)
    }
  }
}

func enableKeyspaceNotifications(ctx context.Context) {
  _ = rdb.ConfigSet(ctx, "notify-keyspace-events", "Ex").Err()
}
