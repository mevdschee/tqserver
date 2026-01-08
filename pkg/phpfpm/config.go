package phpfpm

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/mevdschee/tqserver/pkg/config/php"
)

// GeneratePHPFPMConfig renders a minimal php-fpm main config and a pool config
// from the provided `php.Config`. Files are written into `outDir` and the
// paths of the main config and pool config are returned.
func GeneratePHPFPMConfig(cfg *php.Config, outDir string) (mainConfPath, poolConfPath string, err error) {
	if cfg == nil {
		return "", "", fmt.Errorf("nil config")
	}

	// Ensure output directories exist
	if outDir == "" {
		outDir = os.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return "", "", fmt.Errorf("create outDir: %w", err)
	}
	poolDir := filepath.Join(outDir, "pool.d")
	if err := os.MkdirAll(poolDir, 0o750); err != nil {
		return "", "", fmt.Errorf("create poolDir: %w", err)
	}

	// Render main config
	mainTpl := `[global]
daemonize = no
error_log = {{ .ErrorLog }}
include = {{ .PoolDir }}/*.conf
`

	mainData := map[string]string{
		"ErrorLog": filepath.Join(outDir, "php-fpm.error.log"),
		"PoolDir":  poolDir,
	}

	mainConfPath = filepath.Join(outDir, "php-fpm.conf")
	if err := renderToFile(mainTpl, mainData, mainConfPath); err != nil {
		return "", "", fmt.Errorf("render main conf: %w", err)
	}

	// Prepare pool config data using the new php.Config -> PHPFPM.Pool mapping
	pool := cfg.PHPFPM.Pool
	pm := pool.PM
	if pm == "" {
		pm = "dynamic"
	}

	data := map[string]interface{}{
		"PoolName":       pool.Name,
		"Listen":         cfg.PHPFPM.Listen,
		"PM":             pm,
		"PMIsStatic":     pm == "static",
		"PMIsDynamic":    pm == "dynamic",
		"PMIsOndemand":   pm == "ondemand",
		"MaxChildren":    pool.MaxChildren,
		"StartServers":   pool.StartServers,
		"MinSpare":       pool.MinSpareServers,
		"MaxSpare":       pool.MaxSpareServers,
		"MaxRequests":    pool.MaxRequests,
		"RequestTimeout": fmt.Sprintf("%ds", int(pool.RequestTerminateTimeout.Round(time.Second).Seconds())),
		"IdleTimeout":    fmt.Sprintf("%ds", int(pool.ProcessIdleTimeout.Round(time.Second).Seconds())),
		"DocumentRoot":   cfg.DocumentRoot,
		// Settings are PHP INI-style directives that should be applied as
		// php_admin_flag[...] or php_admin_value[...] in the pool config.
		"Settings": cfg.Settings,
		// Env contains explicit environment variables to export into the pool
		// (rendered as env[...] entries).
		"Env": cfg.PHPFPM.Env,
	}

	poolTpl := `[{{ .PoolName }}]
listen = {{ .Listen }}
pm = {{ .PM }}
{{ if .PMIsStatic }}pm.max_children = {{ .MaxChildren }}
{{ end }}{{ if .PMIsDynamic }}pm.max_children = {{ .MaxChildren }}
pm.start_servers = {{ .StartServers }}
pm.min_spare_servers = {{ .MinSpare }}
pm.max_spare_servers = {{ .MaxSpare }}
{{ end }}{{ if .PMIsOndemand }}pm.max_children = {{ .MaxChildren }}
process_idle_timeout = {{ .IdleTimeout }}
{{ end }}pm.max_requests = {{ .MaxRequests }}
request_terminate_timeout = {{ .RequestTimeout }}
chdir = {{ .DocumentRoot }}
{{/* Render PHP INI directives as php_admin_flag or php_admin_value. */}}
{{ range $k, $v := .Settings }}php_admin_flag[{{ $k }}] = {{ $v }}
{{ end }}
{{ range $k, $v := .Env }}env[{{ $k }}] = {{ $v }}
{{ end }}`

	poolConfPath = filepath.Join(poolDir, pool.Name+".conf")
	if err := renderToFile(poolTpl, data, poolConfPath); err != nil {
		return "", "", fmt.Errorf("render pool conf: %w", err)
	}

	return mainConfPath, poolConfPath, nil
}

func renderToFile(tpl string, data interface{}, path string) error {
	tt, err := template.New("conf").Parse(tpl)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := tt.Execute(f, data); err != nil {
		return err
	}
	return nil
}
