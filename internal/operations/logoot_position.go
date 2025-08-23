package operations

import (
	"encoding/hex"
	"math/big"

	"golang.org/x/crypto/sha3"
)

type PositionKey string

type LogootPosition struct {
	Segments []PositionSegment `json:"segments"`
	Hash     PositionKey       `json:"hash"`
}

type PositionSegment struct {
	Value    *big.Int `json:"value"`
	AuthorID AuthorID `json:"author"`
}

func NewLogootPosition(segments []PositionSegment) LogootPosition {
	pos := LogootPosition{
		Segments: segments,
	}
	pos.computeHash()
	return pos
}

func (p *LogootPosition) computeHash() {
	hasher := sha3.New256()
	for _, segment := range p.Segments {
		hasher.Write(segment.Value.Bytes())
		hasher.Write([]byte(segment.AuthorID))
	}
	hash := hasher.Sum(nil)
	p.Hash = PositionKey(hex.EncodeToString(hash))
}

func (p LogootPosition) Key() PositionKey {
	return p.Hash
}

func (p LogootPosition) Compare(other LogootPosition) int {
	minLen := len(p.Segments)
	if len(other.Segments) < minLen {
		minLen = len(other.Segments)
	}

	for i := 0; i < minLen; i++ {
		cmp := p.Segments[i].Value.Cmp(other.Segments[i].Value)
		if cmp != 0 {
			return cmp
		}

		if p.Segments[i].AuthorID < other.Segments[i].AuthorID {
			return -1
		} else if p.Segments[i].AuthorID > other.Segments[i].AuthorID {
			return 1
		}
	}

	if len(p.Segments) < len(other.Segments) {
		return -1
	} else if len(p.Segments) > len(other.Segments) {
		return 1
	}

	return 0
}

func (p LogootPosition) String() string {
	result := ""
	for i, segment := range p.Segments {
		if i > 0 {
			result += "."
		}
		result += segment.Value.String() + ":" + string(segment.AuthorID)
	}
	return result
}

func (p LogootPosition) IsValid() bool {
	if len(p.Segments) == 0 {
		return false
	}

	for _, segment := range p.Segments {
		if segment.Value == nil || segment.AuthorID == "" {
			return false
		}
	}

	return true
}

func GeneratePosition(left, right LogootPosition, authorID AuthorID) LogootPosition {
	if !left.IsValid() && !right.IsValid() {
		return NewLogootPosition([]PositionSegment{
			{Value: big.NewInt(1), AuthorID: authorID},
		})
	}

	if !left.IsValid() {
		value := new(big.Int).Sub(right.Segments[0].Value, big.NewInt(1))
		if value.Cmp(big.NewInt(0)) <= 0 {
			segments := make([]PositionSegment, len(right.Segments)+1)
			copy(segments[1:], right.Segments)
			segments[0] = PositionSegment{Value: big.NewInt(0), AuthorID: authorID}
			return NewLogootPosition(segments)
		}
		return NewLogootPosition([]PositionSegment{
			{Value: value, AuthorID: authorID},
		})
	}

	if !right.IsValid() {
		value := new(big.Int).Add(left.Segments[0].Value, big.NewInt(1))
		return NewLogootPosition([]PositionSegment{
			{Value: value, AuthorID: authorID},
		})
	}

	return generatePositionBetween(left, right, authorID)
}

func generatePositionBetween(left, right LogootPosition, authorID AuthorID) LogootPosition {
	minLen := len(left.Segments)
	if len(right.Segments) < minLen {
		minLen = len(right.Segments)
	}

	commonPrefixLen := 0
	for i := 0; i < minLen; i++ {
		if left.Segments[i].Value.Cmp(right.Segments[i].Value) != 0 {
			break
		}
		if left.Segments[i].AuthorID != right.Segments[i].AuthorID {
			break
		}
		commonPrefixLen++
	}

	var segments []PositionSegment
	for i := 0; i < commonPrefixLen; i++ {
		segments = append(segments, left.Segments[i])
	}

	if commonPrefixLen < minLen {
		leftVal := left.Segments[commonPrefixLen].Value
		rightVal := right.Segments[commonPrefixLen].Value
		diff := new(big.Int).Sub(rightVal, leftVal)

		if diff.Cmp(big.NewInt(1)) > 0 {
			midVal := new(big.Int).Add(leftVal, new(big.Int).Div(diff, big.NewInt(2)))
			segments = append(segments, PositionSegment{Value: midVal, AuthorID: authorID})
		} else {
			segments = append(segments, left.Segments[commonPrefixLen])
			segments = append(segments, PositionSegment{Value: big.NewInt(1), AuthorID: authorID})
		}
	} else {
		if len(left.Segments) == commonPrefixLen {
			segments = append(segments, PositionSegment{Value: big.NewInt(1), AuthorID: authorID})
		} else {
			segments = append(segments, left.Segments[commonPrefixLen])
			segments = append(segments, PositionSegment{Value: big.NewInt(1), AuthorID: authorID})
		}
	}

	return NewLogootPosition(segments)
}
