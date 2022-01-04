package types

// Yarn ...
type Yarn struct {
	Root Twt
	Twts Twts
}

func (yarn Yarn) GetLastTwt() Twt {
	if len(yarn.Twts) > 0 {
		return yarn.Twts[len(yarn.Twts)-1]
	}
	return yarn.Root
}

// Yarns ...
type Yarns []Yarn

func (yarns Yarns) AsTwts() Twts {
	var twts Twts

	for _, yarn := range yarns {
		twts = append(twts, yarn.GetLastTwt())
	}

	return twts
}

func (yarns Yarns) Len() int {
	return len(yarns)
}
func (yarns Yarns) Less(i, j int) bool {
	if yarns[i].GetLastTwt().Created().Before(yarns[j].GetLastTwt().Created()) {
		return false
	}

	if yarns[i].GetLastTwt().Created().After(yarns[j].GetLastTwt().Created()) {
		return true
	}

	return yarns[i].GetLastTwt().Hash() > yarns[j].GetLastTwt().Hash()
}

func (yarns Yarns) Swap(i, j int) {
	yarns[i], yarns[j] = yarns[j], yarns[i]
}
