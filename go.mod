module xiaozhi-esp32-server-golang

go 1.25.0

require (
	github.com/ThinkInAIXYZ/go-mcp v0.2.19
	github.com/antonfisher/nested-logrus-formatter v1.3.1
	github.com/asaskevich/EventBus v0.0.0-20200907212545-49d423059eef
	github.com/bytedance/sonic v1.15.2
	github.com/cloudwego/eino v0.9.4
	github.com/cloudwego/eino-ext/components/model/ollama v0.1.9
	github.com/cloudwego/eino-ext/components/model/openai v0.1.13
	github.com/difyz9/edge-tts-go v0.0.3
	github.com/eclipse/paho.mqtt.golang v1.5.1
	github.com/eino-contrib/jsonschema v1.0.3
	github.com/getkin/kin-openapi v0.118.0
	github.com/gin-gonic/gin v1.10.1
	github.com/go-audio/audio v1.0.0
	github.com/go-audio/wav v1.1.0
	github.com/golang-jwt/jwt/v4 v4.5.2
	github.com/google/uuid v1.6.0
	github.com/gopxl/beep v1.4.1
	github.com/gorilla/websocket v1.5.3
	github.com/hackers365/go-webrtcvad v0.0.0-20250711024710-dde35479e077
	github.com/hackers365/mem0-go v1.0.2
	github.com/hackers365/silero-vad-go v0.2.2-0.20260521042711-c8860c450795
	github.com/hraban/opus v0.0.0-20220302220929-eeacdbcb92d0
	github.com/lestrrat-go/file-rotatelogs v2.4.0+incompatible
	github.com/mark3labs/mcp-go v0.43.0
	github.com/memodb-io/memobase/src/client/memobase-go v0.0.0-20251008012534-936f45328453
	github.com/mitchellh/hashstructure/v2 v2.0.2
	github.com/mochi-mqtt/server/v2 v2.7.9
	github.com/nats-io/nats.go v1.52.0
	github.com/orcaman/concurrent-map/v2 v2.0.1
	github.com/redis/go-redis/v9 v9.19.0
	github.com/sirupsen/logrus v1.9.4
	github.com/spf13/viper v1.20.1
	github.com/stretchr/testify v1.11.1
	github.com/tmaxmax/go-sse v0.11.0
	go.uber.org/zap v1.27.0
	gopkg.in/hraban/opus.v2 v2.0.0-20230925203106-0188a62cb302
	gorm.io/gorm v1.31.1
	voice_server v0.0.0-00010101000000-000000000000
	xiaozhi/manager/backend v0.0.0-00010101000000-000000000000
)

// 主进程内嵌 manager HTTP 时引用 backend 子模块
replace xiaozhi/manager/backend => ./manager/backend

// 主进程内嵌 asr_server 时引用 asr_server 子模块（Git submodule）
replace voice_server => ./asr_server

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/buger/jsonparser v1.2.0 // indirect
	github.com/bytedance/gopkg v0.1.4 // indirect
	github.com/bytedance/sonic/loader v0.5.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudwego/base64x v0.1.7 // indirect
	github.com/cloudwego/eino-ext/libs/acl/openai v0.1.17 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/eino-contrib/ollama v0.1.0 // indirect
	github.com/evanphx/json-patch v0.5.2 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.3 // indirect
	github.com/gin-contrib/cors v1.7.2 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/glebarez/go-sqlite v1.21.2 // indirect
	github.com/glebarez/sqlite v1.11.0 // indirect
	github.com/go-audio/riff v1.0.0 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/swag v0.19.15 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.20.0 // indirect
	github.com/go-sql-driver/mysql v1.9.3 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/goph/emperror v0.17.2 // indirect
	github.com/hajimehoshi/go-mp3 v0.3.4 // indirect
	github.com/invopop/jsonschema v0.13.0 // indirect
	github.com/invopop/yaml v0.1.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/jonboulle/clockwork v0.5.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/k2-fsa/sherpa-onnx-go v1.12.4 // indirect
	github.com/k2-fsa/sherpa-onnx-go-linux v1.12.4 // indirect
	github.com/k2-fsa/sherpa-onnx-go-macos v1.12.4 // indirect
	github.com/k2-fsa/sherpa-onnx-go-windows v1.12.4 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/lestrrat-go/strftime v1.1.0 // indirect
	github.com/mailru/easyjson v0.9.2 // indirect
	github.com/mattn/go-isatty v0.0.21 // indirect
	github.com/meguminnnnnnnnn/go-openai v0.1.5 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/nats-io/nkeys v0.4.15 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/nikolalohinski/gonja v1.5.3 // indirect
	github.com/pelletier/go-toml/v2 v2.3.1 // indirect
	github.com/perimeterx/marshmallow v1.1.4 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/qdrant/go-client v1.16.2 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/rs/xid v1.4.0 // indirect
	github.com/sagikazarmark/locafero v0.7.0 // indirect
	github.com/slongfield/pyfmt v0.0.0-20220222012616-ea85ff4c361f // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.12.0 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.12 // indirect
	github.com/wk8/go-ordered-map/v2 v2.1.8 // indirect
	github.com/yargevad/filepathx v1.0.0 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	go.opentelemetry.io/otel v1.43.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/arch v0.28.0 // indirect
	golang.org/x/crypto v0.53.0 // indirect
	golang.org/x/exp v0.0.0-20260603202125-055de637280b // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.38.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260420184626-e10c466a9529 // indirect
	google.golang.org/grpc v1.80.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gorm.io/driver/mysql v1.5.6 // indirect
	modernc.org/libc v1.72.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
	modernc.org/sqlite v1.49.1 // indirect
)
