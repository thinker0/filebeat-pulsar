/**
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package main

import (
	"fmt"
	"github.com/elastic/beats/v7/x-pack/filebeat/cmd"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	LOG "github.com/sirupsen/logrus"
	"github.com/thinker0/v2/filebeat-pulsar/pkg/pulsar"
	"github.com/urfave/negroni"
	_ "go.uber.org/automaxprocs"
	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	Version      string = ""
	GitTag       string = ""
	GitCommit    string = ""
	GitTreeState string = ""

	healthy     int32
	adminServer *http.Server

	logger *log.Logger
)

func main() {
    logger = log.New(os.Stderr, "http: ", log.LstdFlags)
    done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	signal.Notify(quit, os.Kill)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)
	adminPort := os.Getenv("ADMIN_PORT")
	// Admin server
	g.Go(func() error {
		ln, err := net.Listen("tcp", adminPort)
		if err != nil {
			LOG.Error("HTTP Admin server: failed to listen", "error", err)
			os.Exit(2)
		}

		shutdownFunc := func() {
			defer cancel()
			if adminServer != nil {
				go func() {
					adminServer.Close()
					adminServer.Shutdown(context.Background())
				}()
			}
		}
		adminRouter := http.NewServeMux()
		adminRouter.Handle("/health", Healthz(&healthy))
		adminRouter.Handle("/metrics", promhttp.Handler())
		adminRouter.Handle("/quitquitquit", QuitQuitQuit(&healthy, shutdownFunc))
		adminRouter.Handle("/abortabortabort", AbortAbortAbort(&healthy, shutdownFunc))
		adminRouter.HandleFunc("/debug/pprof/", pprof.Index)
		adminRouter.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		adminRouter.HandleFunc("/debug/pprof/profile", pprof.Profile)
		adminRouter.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		adminRouter.HandleFunc("/debug/pprof/trace", pprof.Trace)
		adminHandler := negroni.Classic()
		adminHandler.UseHandler(adminRouter)
		adminServer := &http.Server{
			Handler:      adminHandler,
			ErrorLog:     logger,
			ReadTimeout:  120 * time.Second,
			WriteTimeout: 120 * time.Second,
			IdleTimeout:  120 * time.Second,
		}
		atomic.StoreInt32(&healthy, 1)
		adminServer.SetKeepAlivesEnabled(true)
		adminServer.RegisterOnShutdown(func() {
			atomic.StoreInt32(&healthy, 0)
			LOG.Infof("HTTP Admin server shutdown at %s", adminPort)
		})
		LOG.Infof("HTTP Admin server serving at %s", adminPort)

		return adminServer.Serve(ln)
	})


    g.Go(func() error {
        <-quit
        LOG.Println("Server is shutting down...")
        atomic.StoreInt32(&healthy, 0)

        if adminServer != nil {
            go func() {
                adminServer.Close()
                adminServer.Shutdown(context.Background())
            }()
        }
        err := g.Wait()
        if err != nil {
            LOG.Errorf("server returning an error: %v", err)
            os.Exit(2)
        }
        close(done)
        return nil
    })

	pulsar.Init()
	if err := cmd.Filebeat().Execute(); err != nil {
		os.Exit(1)
	}

	select {
	case <-quit:
		break
	case <-ctx.Done():
		break
	}
	log.Println("Server stopped")
}

func Healthz(healthy *int32) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(healthy) == 1 {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintln(w, "ok")
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
}

func QuitQuitQuit(healthy *int32, shutdown func()) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		LOG.Info("Server is QuitQuitQuit...")
		atomic.StoreInt32(healthy, 0)
		defer shutdown()
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "ok")
	})
}

func AbortAbortAbort(healthy *int32, shutdown func()) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		defer shutdown()
		LOG.Info("Server is AbortAbortAbort...")
		atomic.StoreInt32(healthy, 0)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "ok")
	})
}
