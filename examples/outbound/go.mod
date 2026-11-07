module github.com/andywang2011/go-httplib/examples/outbound

go 1.27.3

require (
	github.com/andywang2011/go-httplib v0.0.0
	github.com/andywang2011/go-logging v0.1.0
)

require gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect

replace github.com/andywang2011/go-logging => ../../../go-logging

replace github.com/andywang2011/go-httplib => ../..
