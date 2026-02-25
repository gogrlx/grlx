package service

import "context"

type ServiceProvider interface {
	Properties() (map[string]interface{}, error)
	Parse(id, method string, properties map[string]interface{}) (ServiceProvider, error)

	Start(context.Context) error
	Stop(context.Context) error
	Status(context.Context) (string, error)

	Enable(context.Context) error
	Disable(context.Context) error
	IsEnabled(context.Context) (bool, error)

	IsRunning(context.Context) (bool, error)
	Restart(context.Context) error

	Mask(context.Context) error
	Unmask(context.Context) error
	IsMasked(context.Context) (bool, error)

	InitName() string
	IsInit() bool
}
