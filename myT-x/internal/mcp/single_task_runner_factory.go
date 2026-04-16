package mcp

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	singletaskrunnermcp "myT-x/internal/mcp/single-task-runner"
	"myT-x/internal/singletaskrunner"
)

func singleTaskRunnerRuntimeFactory(sessionName string, svcManager *singletaskrunner.ServiceManager) RuntimeFactory {
	return func(in io.Reader, out io.Writer) (MCPRuntime, error) {
		if svcManager == nil {
			return nil, errors.New("single-task-runner service manager is nil")
		}
		svc := svcManager.GetOrCreate(sessionName)
		if svc == nil {
			return nil, fmt.Errorf("single-task-runner service could not be created for session %q", sessionName)
		}
		return singletaskrunnermcp.NewRuntime(singletaskrunnermcp.Config{
			In:      in,
			Out:     out,
			Logger:  log.New(os.Stderr, "[single-task-runner] ", log.LstdFlags),
			Service: svc,
		})
	}
}
