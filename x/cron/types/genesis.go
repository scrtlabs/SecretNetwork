package types

import "fmt"

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		ScheduleList: []Schedule{},
		Params:       DefaultParams(),
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	// Check for duplicated index in schedule
	scheduleIndexMap := make(map[string]struct{})

	for _, elem := range gs.ScheduleList {
		index := string(GetScheduleKey(elem.Name))
		if _, ok := scheduleIndexMap[index]; ok {
			return fmt.Errorf("duplicated index for schedule")
		}
		scheduleIndexMap[index] = struct{}{}
	}

	return gs.Params.Validate()
}
