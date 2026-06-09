package runtime

import "context"

// StartupTask is a short-running task executed sequentially before services start.
type StartupTask interface {
	Name() string
	Run(ctx context.Context, app *App) error
}

// StartupTaskFunc is a functional adapter implementing StartupTask.
type StartupTaskFunc struct {
	TaskName string
	Fn       func(ctx context.Context, app *App) error
}

func (t *StartupTaskFunc) Name() string { return t.TaskName }

func (t *StartupTaskFunc) Run(ctx context.Context, app *App) error {
	return t.Fn(ctx, app)
}
