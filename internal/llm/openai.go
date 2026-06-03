package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

const systemPrompt = `You are a market-disruption research analyst for a permanently private internal tool.
Your job is to analyse excerpts from public business white papers and reports, then identify market disruption signals and possible SaaS product opportunities.

Rules:
- Ground every claim in the provided text. If you cannot cite the text, do not assert.
- Distinguish "what the source says" (cited) from "our analysis" (clearly labelled inference).
- Reject ideas that are: too generic (no specific buyer), consulting-not-software (fix is advisory, not a recurring workflow), undefended (no moat signal), or ungrounded (no evidence in provided text).
- Produce valid JSON matching the schema requested. All integer scores are 1–10.
- Be conservative: reject more than you accept.`

type OpenAIClient struct {
	client *openai.Client
	model  string
}

func NewOpenAI(apiKey, model string) *OpenAIClient {
	return &OpenAIClient{
		client: openai.NewClient(apiKey),
		model:  model,
	}
}

func (c *OpenAIClient) Triage(ctx context.Context, docMeta string, sampleChunks []string) (TriageResult, error) {
	prompt := fmt.Sprintf(`Document metadata: %s

Sample content:
%s

Reply with JSON:
{"relevant": bool, "reason": "string", "topics": ["string"]}

relevant=true only if this document contains concrete market disruption signals, pain points, or technology/regulatory drivers that could lead to a SaaS opportunity.`, docMeta, strings.Join(sampleChunks, "\n---\n"))

	raw, err := c.complete(ctx, prompt)
	if err != nil {
		return TriageResult{}, err
	}
	var result TriageResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return TriageResult{}, fmt.Errorf("triage parse: %w (raw: %s)", err, raw)
	}
	return result, nil
}

func (c *OpenAIClient) ExtractSignals(ctx context.Context, chunks []string, sourceURL string) ([]DisruptionSignal, error) {
	prompt := fmt.Sprintf(`Source URL: %s

Text chunks (indexed from 0):
%s

Extract all market disruption signals and pain points. For each signal you MUST cite the chunk_refs (0-based index).

Reply with JSON object:
{"signals":[{"summary":"string","signal_type":"technology_shift|regulatory|demand_shift|competitive","pain_point":"string","industries":[],"regions":[],"chunk_refs":[int],"confidence":0.0-1.0}]}

Return {"signals":[]} if no grounded signals found.`, sourceURL, indexedChunks(chunks))

	raw, err := c.complete(ctx, prompt)
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Signals []DisruptionSignal `json:"signals"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapper); err != nil {
		return nil, fmt.Errorf("signals parse: %w (raw: %s)", err, raw)
	}
	return wrapper.Signals, nil
}

func (c *OpenAIClient) GenerateIdeas(ctx context.Context, signals []DisruptionSignal, chunks []string, sourceURL string) ([]CandidateIdea, error) {
	sigJSON, _ := json.Marshal(signals)
	prompt := fmt.Sprintf(`Source URL: %s

Evidence chunks (indexed from 0):
%s

Disruption signals extracted from above chunks:
%s

For each real software opportunity, generate a SaaS idea candidate.
Requirements:
- Must have a specific buyer who controls a budget
- Must be a recurring workflow software can automate or manage (not consulting/advisory)
- Must cite at least one chunk as evidence (use chunk_index from the list above)
- Be concrete: name the target industry, company size, and buyer persona

Sales motion must be one of: enterprise, SMB, mass-market, developer-led, marketplace.
All integer scores (high_ticket_potential … mvp_complexity) are 1–10.
Leave overall_score = 0.
For each citation: use a real excerpt ≤25 words from the chunk text above.

Reply with a JSON object:
{"ideas":[{"idea_name":"","one_sentence_pitch":"","source_documents":[""],"industries":[],"countries_or_regions":[],"disruption_driver":"","pain_point":"","target_customer":"","buyer_persona":"","sales_motion":"","high_ticket_potential":0,"mass_market_potential":0,"technical_feasibility":0,"market_urgency":0,"competition_risk":0,"data_availability":0,"mvp_complexity":0,"overall_score":0,"why_it_might_work":"","why_it_might_fail":"","possible_mvp":"","first_10_customers":"","validation_questions":[],"citations":[{"document_id":"","source_url":"","title":"","publisher":"","published_at":"","excerpt":"","chunk_index":0}]}]}

Return {"ideas":[]} only if there is truly no grounded software opportunity.`, sourceURL, indexedChunks(chunks), string(sigJSON))

	raw, err := c.complete(ctx, prompt)
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Ideas []CandidateIdea `json:"ideas"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapper); err != nil {
		return nil, fmt.Errorf("ideas parse: %w (raw: %s)", err, raw)
	}
	return wrapper.Ideas, nil
}

func (c *OpenAIClient) Score(ctx context.Context, idea CandidateIdea, chunks []string) (SubScores, error) {
	ideaJSON, _ := json.Marshal(idea)
	prompt := fmt.Sprintf(`Rate this SaaS idea on the 14 weighted criteria from docs/06 (all 1–10) plus two penalty criteria.
Evidence chunks for grounding:
%s

Idea:
%s

Reply with JSON:
{"criteria":[14 ints 1-10 in order: problem_frequency,urgency,willingness_to_pay,budget_owner_clarity,regulatory_pressure,competition(higher=worse),market_size,impl_complexity(higher=worse),data_availability,defensibility,sales_motion_clarity,high_ticket_potential,time_to_mvp(higher=worse),founder_fit],"generic_risk":1-10,"consulting_risk":1-10}

Be strict. Use the evidence chunks to justify scores.`, strings.Join(chunks, "\n---\n"), string(ideaJSON))

	raw, err := c.complete(ctx, prompt)
	if err != nil {
		return SubScores{}, err
	}
	var resp struct {
		Criteria       [14]int `json:"criteria"`
		GenericRisk    int     `json:"generic_risk"`
		ConsultingRisk int     `json:"consulting_risk"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return SubScores{}, fmt.Errorf("score parse: %w (raw: %s)", err, raw)
	}
	return SubScores{
		Criteria:       resp.Criteria,
		GenericRisk:    resp.GenericRisk,
		ConsultingRisk: resp.ConsultingRisk,
	}, nil
}

func (c *OpenAIClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	resp, err := c.client.CreateEmbeddings(ctx, openai.EmbeddingRequestStrings{
		Input: texts,
		Model: openai.SmallEmbedding3,
	})
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}
	out := make([][]float32, len(resp.Data))
	for i, d := range resp.Data {
		out[i] = d.Embedding
	}
	return out, nil
}

func (c *OpenAIClient) complete(ctx context.Context, userPrompt string) (string, error) {
	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
		Temperature: 0.2,
	})
	if err != nil {
		return "", fmt.Errorf("openai: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openai: no choices returned")
	}
	return resp.Choices[0].Message.Content, nil
}

func (c *OpenAIClient) CritiqueIdeas(ctx context.Context, ideas []CandidateIdea) ([]CritiqueResult, error) {
	ideasJSON, _ := json.Marshal(ideas)
	prompt := fmt.Sprintf(`Apply the rejection filters from docs/07 §4 to each idea:
- Too generic (no specific buyer/vertical)
- Consulting-not-software (advisory fix, not recurring workflow)
- No budget owner
- Solved/commoditized (strong incumbents, no wedge)
- Not data/workflow-shaped
- Ungrounded (pain point has no cited evidence)

Ideas (0-indexed):
%s

Reply with JSON object:
{"critiques":[{"idea_index":0,"keep":true,"reason":"string"}]}

One entry per idea. Be conservative — reject marginal ideas.`, string(ideasJSON))

	raw, err := c.complete(ctx, prompt)
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Critiques []CritiqueResult `json:"critiques"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapper); err != nil {
		return nil, fmt.Errorf("critique parse: %w (raw: %s)", err, raw)
	}
	return wrapper.Critiques, nil
}

func (c *OpenAIClient) LabelIdea(ctx context.Context, idea CandidateIdea) (string, error) {
	ideaJSON, _ := json.Marshal(idea)
	prompt := fmt.Sprintf(`Assign exactly one label from this taxonomy to the SaaS idea:
- enterprise_high_ticket
- smb_saas
- vertical_saas
- developer_tool
- compliance_regtech
- ai_workflow_automation
- marketplace
- consumer_mass_market
- not_suitable

Idea:
%s

Reply with JSON: {"label":"<one of the 9 above>","rationale":"string"}`, string(ideaJSON))

	raw, err := c.complete(ctx, prompt)
	if err != nil {
		return "", err
	}
	var resp struct {
		Label string `json:"label"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return "", fmt.Errorf("label parse: %w (raw: %s)", err, raw)
	}
	if resp.Label == "" {
		return "not_suitable", nil
	}
	return resp.Label, nil
}

func indexedChunks(chunks []string) string {
	var sb strings.Builder
	for i, c := range chunks {
		fmt.Fprintf(&sb, "[%d] %s\n", i, c)
	}
	return sb.String()
}
