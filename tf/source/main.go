package main

import (
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"log/slog"
	"os"
	"strings"
)

var logger = slog.New(slog.NewTextHandler(os.Stderr, nil))

type Handler struct {
	client  Client
	actions map[string]func() error
}

func NewHandler(asgPrefixes []string) (Handler, error) {
	client, err := NewClient(asgPrefixes)
	if err != nil {
		return Handler{}, fmt.Errorf("new aws client: %w", err)
	}

	h := Handler{client: client}
	h.actions = make(map[string]func() error)
	h.actions["scale-up"] = h.scaleUp
	h.actions["scale-down"] = h.scaleDown
	return h, nil
}

func (h Handler) HandleRequest(in interface{}) error {
	request, ok := in.(map[string]interface{})
	if !ok {
		return fmt.Errorf("unexpected %T request %+v", in, in)
	}

	actionKey := strings.TrimSpace(fmt.Sprintf("%s", request["Payload"]))
	action, ok := h.actions[actionKey]
	if !ok {
		return fmt.Errorf("unrecognized %s request action", actionKey)
	}

	logger.Info(fmt.Sprintf("received %s request action", actionKey))
	return action()
}

func (h Handler) scaleUp() error {
	logger.Info("scaling up")
	return h.client.ScaleUp()
}

func (h Handler) scaleDown() error {
	logger.Info("scaling down")
	return h.client.ScaleDown()
}

func main() {
	prefixes, ok := os.LookupEnv("ASG_PREFIX")
	if !ok {
		logger.Error(fmt.Sprintf("ASG_PREFIX env. var not set, these are comma seperated prefixes of autoscaling groupus"))
		os.Exit(1)
	}

	handler, err := NewHandler(strings.Split(prefixes, ","))
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	lambda.Start(handler.HandleRequest)
}
