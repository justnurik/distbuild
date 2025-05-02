package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"context"

	"fmt"
	"sync"

	"gitlab.com/justnurik/distbuild/bin"
	"gitlab.com/justnurik/distbuild/pkg/build"
	"gitlab.com/justnurik/distbuild/pkg/client"
)

type args struct {
	root string

	logPath   string
	sourceDir string

	coordinatorEndpoint string

	debug bool
}

func (a *args) parse() {
	flag.StringVar(&a.root, "root", ".", "prefix all path")
	flag.StringVar(&a.logPath, "log-file", "logs", "log file path")
	flag.StringVar(&a.sourceDir, "source-dir", ".", "dir with surce files")
	flag.StringVar(&a.coordinatorEndpoint, "coordinator", "", "coordinator endpoint")
	flag.BoolVar(&a.debug, "debug", false, "debug version")

	flag.Parse()

	a.logPath = filepath.Join(a.root, a.logPath)
	a.sourceDir = filepath.Join(a.root, a.sourceDir)
}

func main() {
	var a args
	a.parse()

	logger, err := bin.NewLogger(a.logPath, a.debug)
	if err != nil {
		log.Fatal("new logger create fail")
	}

	client := client.NewClient(logger, a.coordinatorEndpoint, a.sourceDir)
	simple(client)
}

func simple(client *client.Client) {
	sem := make(chan struct{}, 20) // Ограничение 20 одновременных запросов

	buildCount := 100

	var wg sync.WaitGroup
	wg.Add(buildCount)

	for i := range buildCount {

		var echoGraph = build.Graph{
			Jobs: []build.Job{
				{
					ID:   build.ID{'1', byte(i)},
					Name: "echo",
					Cmds: []build.Cmd{
						{Exec: []string{"echo", "OK"}},
					},
				},
				{
					ID:   build.ID{'2', byte(i)},
					Name: "echo",
					Cmds: []build.Cmd{
						{Exec: []string{"echo", "OK"}},
					},
					Deps: []build.ID{{'1', byte(i)}},
				},
				{
					ID:   build.ID{'3', byte(i)},
					Name: "echo",
					Cmds: []build.Cmd{
						{Exec: []string{"echo", "OK"}},
					},
					Deps: []build.ID{{'1', byte(i)}, {'2', byte(i)}},
				},
			},
		}

		sem <- struct{}{}
		go func(i int) {
			defer func() { <-sem }()
			defer wg.Done()

			ctx := context.Background()
			if err := client.Build(ctx, echoGraph, &lsn{}); err != nil {
				log.Fatal("build err")
			}
		}(i)
	}

	wg.Wait()
}

func test(client *client.Client) {

	baseJob := build.Job{
		ID:   build.ID{'a'},
		Name: "write",
		Cmds: []build.Cmd{
			{CatTemplate: "OK", CatOutput: "{{.OutputDir}}/out.txt"},
		},
	}

	buildCount := 200
	var wg sync.WaitGroup
	wg.Add(buildCount)

	for i := range buildCount {
		depJobID := build.ID{'b', byte(i)}
		depJob := build.Job{
			ID:   depJobID,
			Name: "cat",
			Cmds: []build.Cmd{
				{Exec: []string{"cat", fmt.Sprintf("{{index .Deps %q}}/out.txt", build.ID{'a'})}},
				{Exec: []string{"sleep", "1"}, Environ: os.Environ()}, // DepTimeout is 100ms.
			},
			Deps: []build.ID{{'a'}},
		}

		graph := build.Graph{Jobs: []build.Job{baseJob, depJob}}
		go func() {
			defer wg.Done()

			ctx := context.Background()
			if err := client.Build(ctx, graph, &lsn{}); err != nil {
				return
			}
		}()
	}

	wg.Wait()
}

type lsn struct {
}

func (l *lsn) OnJobStdout(jobID build.ID, stdout []byte) error {
	fmt.Fprintf(os.Stderr, "%s\n", "id: "+jobID.String()+" stderr\n"+string(stdout))
	return nil
}
func (l *lsn) OnJobStderr(jobID build.ID, stderr []byte) error {
	fmt.Fprintf(os.Stderr, "%s\n", "id: "+jobID.String()+" stdout:\n"+string(stderr))
	return nil
}

func (l *lsn) OnJobFinished(jobID build.ID) error {
	fmt.Fprintf(os.Stderr, "%s\n", "id: "+jobID.String()+" job finished")
	return nil
}
func (l *lsn) OnJobFailed(jobID build.ID, code int, err string) error {
	fmt.Fprintf(os.Stderr, "id: %s [job failed] <code: %d, err: %s>\n", jobID.String(), code, err)
	return nil
}
