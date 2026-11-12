package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/javiermolinar/stremio-trakt-export/internal/cinemeta"
	"github.com/javiermolinar/stremio-trakt-export/internal/stremio"
	"github.com/javiermolinar/stremio-trakt-export/internal/webapp"
)

func runServe(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("stremio-trakt-export serve", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	address := flags.String("addr", "127.0.0.1:8080", "loopback address for the local web interface")
	timeout := flags.Duration("http-timeout", 30*time.Second, "timeout for outbound HTTP requests")
	noOpen := flags.Bool("no-open", false, "do not open the web interface in a browser")
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "Usage: %s [--addr 127.0.0.1:8080] [--no-open]\n\n", flags.Name())
		fmt.Fprintln(flags.Output(), "No environment variables are required. Account setup happens in the local web page.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if *timeout <= 0 {
		return fmt.Errorf("--http-timeout must be greater than zero")
	}
	host, _, err := net.SplitHostPort(*address)
	if err != nil {
		return fmt.Errorf("invalid --addr: %w", err)
	}
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return fmt.Errorf("serve mode only supports localhost, 127.0.0.1, or ::1; do not expose account credentials on a network interface")
	}

	httpClient := &http.Client{Timeout: *timeout}
	application := webapp.NewServer()
	application.Stremio = &stremio.Client{HTTPClient: httpClient}
	application.Metadata = &cinemeta.Client{HTTPClient: httpClient}
	application.HTTPClient = httpClient
	// Windows token persistence needs DPAPI and NTFS-specific file handling.
	// Until that is implemented, keep Windows sessions in memory rather than
	// writing unprotected tokens or relying on Unix rename/permission behavior.
	if runtime.GOOS != "windows" {
		application.Store = webapp.NewCredentialStore("")
	}

	listener, err := net.Listen("tcp", *address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", *address, err)
	}
	server := &http.Server{
		Handler:           application.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	shutdownDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = server.Shutdown(shutdownContext)
		case <-shutdownDone:
		}
	}()
	url := "http://" + listener.Addr().String()
	fmt.Fprintf(os.Stderr, "Open %s\n", url)
	if application.Store != nil {
		fmt.Fprintf(os.Stderr, "Credentials are saved with user-only permissions in %s\n", application.Store.Path())
	} else {
		fmt.Fprintln(os.Stderr, "Credentials are held in memory for this session only.")
	}
	if !*noOpen {
		go func() {
			time.Sleep(250 * time.Millisecond)
			if err := openBrowser(url); err != nil {
				fmt.Fprintf(os.Stderr, "Could not open a browser automatically: %v\n", err)
			}
		}()
	}
	err = server.Serve(listener)
	close(shutdownDone)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func openBrowser(url string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", url)
	case "linux":
		command = exec.Command("xdg-open", url)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}
	if err := command.Run(); err != nil {
		return fmt.Errorf("launch browser: %w", err)
	}
	return nil
}
