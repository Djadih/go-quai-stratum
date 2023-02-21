module github.com/Djadih/go-quai-stratum

go 1.16

replace github.com/dominant-strategies/go-quai => ../../djadih/go-quai

replace github.com/dominant-strategies/go-quai-stratum => ../../djadih/go-quai-stratum

require (
	github.com/INFURA/go-ethlibs v0.0.0-20230210163729-fc6ca4235802
	github.com/J-A-M-P-S/structs v1.1.0
	github.com/dominant-strategies/go-quai v0.2.0-rc.0
	github.com/dominant-strategies/go-quai-stratum v0.0.0-20221213185138-b6502d362038
	github.com/ethereum/go-ethereum v1.10.3
	github.com/garyburd/redigo v1.6.4 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/robfig/cron v1.2.0
	gopkg.in/bsm/ratelimit.v1 v1.0.0-20170922094635-f56db5e73a5e // indirect
	gopkg.in/redis.v3 v3.6.4
)
