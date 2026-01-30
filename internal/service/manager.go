package service

import (
	"runtime"
)

type Service interface {
	Install() error
	Uninstall() error
	Start() error
	Stop() error
	Status() (string, error)
}

func NewService() Service {
	switch runtime.GOOS {
	case "linux":
		return &linuxService{}
	case "darwin":
		return &darwinService{}
	case "windows":
		return &windowsService{}
	default:
		return &unsupportedService{}
	}
}

type unsupportedService struct{}

func (u *unsupportedService) Install() error {
	return nil
}

func (u *unsupportedService) Uninstall() error {
	return nil
}

func (u *unsupportedService) Start() error {
	return nil
}

func (u *unsupportedService) Stop() error {
	return nil
}

func (u *unsupportedService) Status() (string, error) {
	return "Unsupported OS", nil
}
