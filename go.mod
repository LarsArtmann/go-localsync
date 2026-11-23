module github.com/larsartmann/go-localsync

go 1.27.2

require (
	charm.land/log/v2 v2.0.1
	github.com/danielgtaylor/huma/v2 v2.41.0
	github.com/larsartmann/go-branded-id v0.7.1
	github.com/larsartmann/go-codec v0.3.3
	github.com/larsartmann/go-cqrs-lite/command/v4 v4.14.1
	github.com/larsartmann/go-cqrs-lite/decider/v4 v4.8.0
	github.com/larsartmann/go-cqrs-lite/event/v4 v4.15.2
	github.com/larsartmann/go-cqrs-lite/id/v4 v4.7.5
	github.com/larsartmann/go-cqrs-lite/middleware/v4 v4.7.2
	github.com/larsartmann/go-cqrs-lite/otel/v4 v4.5.3
	github.com/larsartmann/go-cqrs-lite/projection/v4 v4.4.2
	github.com/larsartmann/go-cqrs-lite/projectionhost/v4 v4.7.1
	github.com/larsartmann/go-cqrs-lite/scenario/v4 v4.4.3
	github.com/larsartmann/go-cqrs-lite/schema/v4 v4.7.1
	github.com/larsartmann/go-cqrs-lite/snapshot/v4 v4.7.0
	github.com/larsartmann/go-cqrs-lite/storage/memory/v4 v4.6.2
	github.com/larsartmann/go-cqrs-lite/storage/v4 v4.11.1
	github.com/larsartmann/go-cqrs-lite/watermill/v4 v4.7.0
	github.com/larsartmann/go-error-family v0.12.0
	github.com/oklog/ulid/v2 v2.1.2
	go.opentelemetry.io/otel v1.47.0
	go.opentelemetry.io/otel/metric v1.47.0
	go.opentelemetry.io/otel/sdk v1.47.0
	go.opentelemetry.io/otel/sdk/metric v1.47.0
	go.opentelemetry.io/otel/trace v1.47.0
	modernc.org/sqlite v1.62.0
)

require (
	charm.land/lipgloss/v2 v2.0.6 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20261013165250-51075c6ea6bc // indirect
	github.com/Microsoft/go-winio v0.6.4 // indirect
	github.com/ThreeDotsLabs/watermill v1.5.3 // indirect
	github.com/bits-and-blooms/bitset v1.29.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/charmbracelet/colorprofile v0.4.3 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20261023060817-a45b7c86333f // indirect
	github.com/charmbracelet/x/ansi v0.11.9 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/ebitengine/purego v0.11.3 // indirect
	github.com/failsafe-go/failsafe-go v0.9.8 // indirect
	github.com/fxamacker/cbor/v2 v2.10.0 // indirect
	github.com/gkampitakis/go-snaps v0.5.25 // indirect
	github.com/go-logfmt/logfmt v0.6.1 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/pprof v0.0.0-20261105003125-4bf881d7fb0b // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgx/v5 v5.12.0 // indirect
	github.com/larsartmann/go-cqrs-lite/dedup/v4 v4.3.0 // indirect
	github.com/larsartmann/go-cqrs-lite/dispatcher/v4 v4.5.2 // indirect
	github.com/larsartmann/go-cqrs-lite/kv/v4 v4.3.5 // indirect
	github.com/larsartmann/go-cqrs-lite/listing/v4 v4.4.4 // indirect
	github.com/larsartmann/go-cqrs-lite/metadata/v4 v4.7.3 // indirect
	github.com/larsartmann/go-cqrs-lite/metaengine/projectionadapter/v4 v4.6.2 // indirect
	github.com/larsartmann/go-cqrs-lite/metaengine/v4 v4.26.0 // indirect
	github.com/larsartmann/go-cqrs-lite/query/v4 v4.10.1 // indirect
	github.com/larsartmann/go-cqrs-lite/record/v4 v4.6.2 // indirect
	github.com/larsartmann/go-cqrs-lite/scheduling/v4 v4.7.0 // indirect
	github.com/larsartmann/go-flightrecorder v0.2.1 // indirect
	github.com/larsartmann/go-idempotency v0.6.0 // indirect
	github.com/larsartmann/go-retry v0.9.0 // indirect
	github.com/larsartmann/go-sqlitestore v0.1.1 // indirect
	github.com/larsartmann/go-sse v0.7.0 // indirect
	github.com/lithammer/shortuuid/v3 v3.0.7 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.1 // indirect
	github.com/lufia/plan9stats v0.0.0-20261101121723-be84dd9509a0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/mattn/go-runewidth v0.0.31 // indirect
	github.com/maypok86/otter/v2 v2.4.0 // indirect
	github.com/moby/sys/user v0.5.0 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/ncruces/go-strftime v1.1.0 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/onsi/ginkgo/v2 v2.34.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/shirou/gopsutil/v4 v4.26.10 // indirect
	github.com/sony/gobreaker v1.0.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xo/terminfo v1.3.1 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.47.0 // indirect
	go.opentelemetry.io/otel/log v1.47.0 // indirect
	golang.org/x/exp v0.0.0-20261014143641-f0251eb4aa3e // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261104224025-d1e00e9f33ab // indirect
	google.golang.org/grpc v1.85.0 // indirect
	modernc.org/libc v1.80.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
)
