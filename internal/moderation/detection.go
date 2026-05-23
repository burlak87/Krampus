package moderation

import "strings"

type Detector struct{}

func NewDetector() *Detector {
	return &Detector{}
}

func (d *Detector) DetectSpamScore(
	content string,
) float64 {

	score := 0.0

	if strings.Count(
		content,
		"http",
	) > 3 {

		score += 5
	}

	if len(content) > 5000 {
		score += 3
	}

	return score
}
