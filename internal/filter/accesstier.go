package filter

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/internal"
)

type AccessTierFilter struct {
	opr  string
	tier string
}

func (filter AccessTierFilter) Apply(fileInfo *internal.ObjAttr) bool {
	// fmt.Println("AccessTier filter ", filter, " file name ", (*fileInfo).Name)  DEBUG PRINT
	fmt.Println("inside filter tier ", filter, " with given tier ", filter.tier, " and file tier ", fileInfo.Tier)
	if (filter.opr == "=") && (filter.tier == strings.ToLower(fileInfo.Tier)) {
		return true
	} else if (filter.opr == "!=") && (filter.tier != strings.ToLower(fileInfo.Tier)) {
		return true
	}
	return false
}

// used for dynamic creation of AccessTierFilter
func newAccessTierFilter(args ...interface{}) Filter {
	return AccessTierFilter{
		opr:  args[0].(string),
		tier: args[1].(string),
	}
}

func giveAccessTierFilterObj(singleFilter *string) (Filter, error) {
	(*singleFilter) = strings.Map(StringConv, (*singleFilter)) //remove all spaces and make all upperCase to lowerCase
	sinChk := (*singleFilter)[4:5]                             //single char after tier (ex- tier=hot , here sinChk will be "=")
	doubChk := (*singleFilter)[4:6]                            //2 chars after tier (ex- tier != cold , here doubChk will be "!=")
	erro := errors.New("invalid filter, no files passed")
	if !((sinChk == "=") || (doubChk == "!=")) {
		return nil, erro
	}
	if (doubChk == "!=") && (len(*singleFilter) > 6) {
		value := (*singleFilter)[6:] // 5 is used since len(tier) = 4 and + 1
		return newAccessTierFilter(doubChk, value), nil
	} else if (sinChk == "=") && (len(*singleFilter) > 5) {
		value := (*singleFilter)[5:] // 5 is used since len(tier) = 4 and + 1
		return newAccessTierFilter(sinChk, value), nil
	} else {
		return nil, erro
	}
}
