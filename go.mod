module github.com/Djadih/go-quai-stratum

go 1.19

replace github.com/dominant-strategies/go-quai => ../../djadih/go-quai

replace github.com/dominant-strategies/go-quai-stratum => ../../djadih/go-quai-stratum

require (
	github.com/INFURA/go-ethlibs v0.0.0-20230210163729-fc6ca4235802
	github.com/J-A-M-P-S/structs v1.1.0
	github.com/dominant-strategies/go-quai v0.2.0-rc.0
	github.com/dominant-strategies/go-quai-stratum v0.0.0-20221213185138-b6502d362038
	github.com/ethereum/go-ethereum v1.10.3
	github.com/gorilla/mux v1.8.0
	github.com/robfig/cron v1.2.0
	gopkg.in/redis.v3 v3.6.4
)

require (
	github.com/btcsuite/btcd v0.20.1-beta // indirect
	github.com/go-stack/stack v1.8.0 // indirect
	github.com/hashicorp/golang-lru v0.5.5-0.20210104140557-80c98217689d // indirect
	github.com/ledgerwatch/secp256k1 v1.0.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	golang.org/x/crypto v0.0.0-20220722155217-630584e8d5aa // indirect
	golang.org/x/sys v0.0.0-20220731174439-a90be440212d // indirect
	gopkg.in/bsm/ratelimit.v1 v1.0.0-20170922094635-f56db5e73a5e // indirect
)
