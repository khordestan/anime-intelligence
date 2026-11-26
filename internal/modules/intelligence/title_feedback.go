package intelligence

type TitleFeedback struct {
	Direction int
	Features  []FeatureValue
}

// ApplyTitleFeedback adds a fallback for features without declared or rating
// signals. Each supporting title contributes at most once per feature.
func (p *TasteProfile) ApplyTitleFeedback(feedback []TitleFeedback) {
	totals := map[string]float64{}
	p.FeedbackCounts = map[string]int{}
	p.Feedback = map[string]Preference{}
	for _, title := range feedback {
		if title.Direction != 1 && title.Direction != -1 {
			continue
		}
		best := map[string]float64{}
		for _, feature := range title.Features {
			if feature.Key == "" || !validUnit(feature.Value) || !validUnit(feature.Confidence) {
				continue
			}
			best[feature.Key] = max(best[feature.Key], feature.Value*feature.Confidence)
		}
		for key, strength := range best {
			_, declared := p.Explicit[key]
			_, inferred := p.Inferred[key]
			if strength > 0 && !declared && !inferred {
				totals[key] += float64(title.Direction) * strength
				p.FeedbackCounts[key]++
			}
		}
	}
	for key, total := range totals {
		count := float64(p.FeedbackCounts[key])
		p.Feedback[key] = Preference{Weight: total / count, Confidence: min(count/6, 0.5)}
	}
}
