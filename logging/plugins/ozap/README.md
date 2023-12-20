ozap
===

ozap is a logging plug-in to wrap [zap](https://github.com/uber-go/zap)
as a [`logging.Logger`](https://pkg.go.dev/github.com/open-policy-agent/opa/logging#Logger).

```golang
opa, err := sdk.New(context.Background(), sdk.Options{
	Logger: ozap.Wrap(logger, level),
})
```
