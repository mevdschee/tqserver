package phpfpm

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mevdschee/tqserver/pkg/config/php"
)

// Launcher controls a supervised php-fpm process started with generated config.
type Launcher struct {
	cfg       *php.Config
	outDir    string
	mainConf  string
	poolConf  string
	cmd       *exec.Cmd
	ctx       context.Context
	cancel    context.CancelFunc
	stoppedCh chan error
}

// NewLauncher creates a Launcher for the given php.Config. If cfg.PHPFPM.GeneratedConfigDir
// is empty, a temp dir will be used.
func NewLauncher(cfg *php.Config) *Launcher {
	out := cfg.PHPFPM.GeneratedConfigDir
	if out == "" {
		out = filepath.Join(os.TempDir(), "tqserver-phpfpm")
	}
	return &Launcher{
		cfg:       cfg,
		outDir:    out,
		stoppedCh: make(chan error, 1),
	}
}

// Start generates php-fpm config files and starts php-fpm with -F -y <mainConf>.
func (l *Launcher) Start() error {
	if l.cfg == nil {
		return fmt.Errorf("nil php config")
	}
	if !l.cfg.PHPFPM.Enabled {
		return fmt.Errorf("php-fpm not enabled in config")
	}

	// generate configs
	main, pool, err := GeneratePHPFPMConfig(l.cfg, l.outDir)
	if err != nil {
		return fmt.Errorf("generate php-fpm config: %w", err)
	}
	l.mainConf = main
	l.poolConf = pool

	bin := l.cfg.PHPFPMBinary
	if bin == "" {
		bin = "php-fpm"
	}

	args := []string{"-y", l.mainConf}
	if l.cfg.PHPIni != "" {
		args = append(args, "-c", l.cfg.PHPIni)
	}
	if l.cfg.PHPFPM.NoDaemonize {
		args = append([]string{"-F"}, args...)
	}

	l.ctx, l.cancel = context.WithCancel(context.Background())
	l.cmd = exec.CommandContext(l.ctx, bin, args...)

	// inherit environment and augment with any configured env
	env := os.Environ()
	for k, v := range l.cfg.PHPFPM.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	l.cmd.Env = env

	// attach stdout/stderr for visibility
	stdout, _ := l.cmd.StdoutPipe()
	stderr, _ := l.cmd.StderrPipe()

	if err := l.cmd.Start(); err != nil {
		return fmt.Errorf("start php-fpm: %w", err)
	}

	log.Printf("[phpfpm] started (pid=%d) using %s", l.cmd.Process.Pid, l.mainConf)

	// stream logs
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			log.Printf("[phpfpm stdout] %s", scanner.Text())
		}
	}()
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Printf("[phpfpm stderr] %s", scanner.Text())
		}
	}()

	// monitor exit
	go func() {
		err := l.cmd.Wait()
		if err != nil {
			log.Printf("[phpfpm] exited with error: %v", err)
		} else {
			log.Printf("[phpfpm] exited")
		}
		l.stoppedCh <- err
	}()

	return nil
}

// Stop requests php-fpm to terminate and waits for it to exit, then removes generated files.
func (l *Launcher) Stop(timeout time.Duration) error {
	if l.cmd == nil || l.cmd.Process == nil {
		return nil
	}

	// attempt graceful shutdown
	if err := l.cmd.Process.Signal(os.Interrupt); err != nil {
		log.Printf("[phpfpm] failed to send interrupt: %v", err)
	}

	select {
	case err := <-l.stoppedCh:
		l.cleanup()
		return err
	case <-time.After(timeout):
		// force kill
		if killErr := l.cmd.Process.Kill(); killErr != nil {
			return fmt.Errorf("failed to kill php-fpm: %w", killErr)
		}
		// wait after killing
		select {
		case err := <-l.stoppedCh:
			l.cleanup()
			return err
		case <-time.After(2 * time.Second):
			l.cleanup()
			return fmt.Errorf("php-fpm did not exit after kill")
		}
	}
}

func (l *Launcher) cleanup() {
	// cancel context
	if l.cancel != nil {
		l.cancel()
	}
	// remove generated configs if they were written into a temp dir
	// only remove if path contains os.TempDir() to avoid deleting user-specified dirs
	if l.outDir == "" {
		return
	}
	if strings.HasPrefix(l.outDir, os.TempDir()) {
		_ = os.RemoveAll(l.outDir)
		log.Printf("[phpfpm] removed generated config dir %s", l.outDir)
	}
}
