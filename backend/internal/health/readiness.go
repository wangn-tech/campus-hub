package health

import (
	"context"
	"time"
)

type Checker interface {
	Name() string
	Check(context.Context) error
}

type CheckFunc struct {
	CheckName string
	Fn        func(context.Context) error
}

func (f CheckFunc) Name() string                    { return f.CheckName }
func (f CheckFunc) Check(ctx context.Context) error { return f.Fn(ctx) }

type Status struct {
	Ready        bool
	Dependencies map[string]string
	Errors       map[string]error
}

type Service struct {
	timeout  time.Duration
	checkers []Checker
}

func New(timeout time.Duration, checkers ...Checker) *Service {
	return &Service{timeout: timeout, checkers: checkers}
}

func (s *Service) Check(ctx context.Context) Status {
	checkCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	status := Status{Dependencies: make(map[string]string, len(s.checkers)), Errors: make(map[string]error)}
	for _, checker := range s.checkers {
		status.Dependencies[checker.Name()] = "down"
	}

	type result struct {
		name string
		err  error
	}
	results := make(chan result, len(s.checkers))
	for _, checker := range s.checkers {
		go func(checker Checker) {
			results <- result{name: checker.Name(), err: checker.Check(checkCtx)}
		}(checker)
	}

	for received := 0; received < len(s.checkers); received++ {
		select {
		case result := <-results:
			if result.err == nil {
				status.Dependencies[result.name] = "up"
				continue
			}
			status.Errors[result.name] = result.err
		case <-checkCtx.Done():
			for name, dependencyStatus := range status.Dependencies {
				if dependencyStatus == "down" {
					status.Errors[name] = checkCtx.Err()
				}
			}
			status.Ready = false
			return status
		}
	}
	status.Ready = len(status.Errors) == 0
	return status
}
