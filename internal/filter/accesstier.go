package filter

import (
	"errors"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/internal"
)

const lenTier = len(tier)

type AccessTierFilter struct {
	opr  bool // true means equal to , false means not equal to
	tier string
}

func (filter AccessTierFilter) Apply(fileInfo *internal.ObjAttr) bool {
	// fmt.Println("AccessTier filter ", filter, " file name ", (*fileInfo).Name)  DEBUG PRINT
	return (filter.opr == (filter.tier == strings.ToLower(fileInfo.Tier))) //if both are same then return true
}

// used for dynamic creation of AccessTierFilter
func newAccessTierFilter(args ...interface{}) Filter {
	return AccessTierFilter{
		opr:  args[0].(bool),
		tier: args[1].(string),
	}
}

func giveAccessTierFilterObj(singleFilter *string) (Filter, error) {
	(*singleFilter) = strings.Map(StringConv, (*singleFilter)) //remove all spaces and make all upperCase to lowerCase
	sinChk := (*singleFilter)[lenTier : lenTier+1]             //single char after tier (ex- tier=hot , here sinChk will be "=")
	doubChk := (*singleFilter)[lenTier : lenTier+2]            //2 chars after tier (ex- tier != cold , here doubChk will be "!=")
	erro := errors.New("invalid accesstier filter, no files passed")
	if !((sinChk == "=") || (doubChk == "!=")) {
		return nil, erro
	}
	if (doubChk == "!=") && (len(*singleFilter) > lenTier+2) {
		value := (*singleFilter)[lenTier+2:] // len(tier) + 2 = 4 and + 2
		return newAccessTierFilter(false, value), nil
	} else if (sinChk == "=") && (len(*singleFilter) > lenTier+1) {
		value := (*singleFilter)[lenTier+1:] // len(tier) + 1 = 4 and + 1
		return newAccessTierFilter(true, value), nil
	} else {
		return nil, erro
	}
}
