package expand

import (
	"context"
	"testing"

	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llm/mock"
)

func TestGenerateProgressPhaseBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name  string
		in    llm.Chatter
		want  []ProgressEvent
		fails bool
	}{
		{name: "success", in: &mockChatter3Phases{researchResp: `{"findings":[],"candidates":[]}`, draftResp: "prose", validateResp: `{"revised_markdown":"prose","claims":[]}`}, want: []ProgressEvent{{Phase: PhaseResearch, Percent: 0}, {Phase: PhaseResearch, Percent: 100}, {Phase: PhaseDraft, Percent: 0}, {Phase: PhaseDraft, Percent: 100}, {Phase: PhaseValidate, Percent: 0}, {Phase: PhaseValidate, Percent: 100}}},
		{name: "research failure", in: mock.NewError(context.DeadlineExceeded), want: []ProgressEvent{{Phase: PhaseResearch, Percent: 0}}, fails: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []ProgressEvent
			_, err := Generate(context.Background(), tc.in, nil, ExpandInput{ArchetypeID: "ontology-epistemology-practice", Topic: "T", ChapterTitle: "C"}, func(e ProgressEvent) { got = append(got, e) })
			if (err != nil) != tc.fails {
				t.Fatalf("error=%v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("events=%+v", got)
			}
			for i, want := range tc.want {
				if got[i].Phase != want.Phase || got[i].Percent != want.Percent || got[i].Message == "" {
					t.Errorf("event %d=%+v want phase=%v percent=%d", i, got[i], want.Phase, want.Percent)
				}
			}
		})
	}
}
