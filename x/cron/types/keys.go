package types

const (
	// ModuleName defines the module name
	ModuleName = "cron"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_cron"
)

const (
	prefixScheduleKey = iota + 1
	prefixScheduleCountKey
	prefixParamsKey
)

var (
	ScheduleKey      = []byte{prefixScheduleKey}
	ScheduleCountKey = []byte{prefixScheduleCountKey}
	ParamsKey        = []byte{prefixParamsKey}
)

func GetScheduleKey(name string) []byte {
	return []byte(name)
}
