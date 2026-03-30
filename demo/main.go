// demo/main.go — Compares token usage with and without Engram identity serialization.
//
// Usage:
//
//	go run ./demo
//
// Requires OPENAI_API_KEY env var (reads from ../api.txt if present).
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Identity: the verbose prose a client would normally paste into every prompt.
// ---------------------------------------------------------------------------

const proseIdentity = `You are acting as an expert advisor for Captain Michael Anthony Torres of the Springfield Metropolitan Fire Department, Station 7, Engine Company 7. Captain Torres has been serving in the fire service for exactly 20 years, having started as a probationary firefighter in 2006 and working his way through the ranks of Firefighter, Senior Firefighter, Lieutenant, and finally Captain, which he achieved in 2019. He is 44 years old and plans to retire in approximately 5 years.

Captain Torres's primary operational domain is structural fire suppression, but he has extensive cross-training and experience in multiple specialty areas. His current certifications include: Hazardous Materials Operations (HazMat Ops) certified through the International Fire Service Accreditation Congress (IFSAC), Technical Rescue Technician with specializations in confined space rescue and rope rescue certified through the National Board on Fire Service Professional Qualifications (Pro Board), Fire Instructor II certified through IFSAC, Fire Officer II certified through Pro Board, Emergency Medical Technician - Basic (EMT-B) certified through the National Registry of Emergency Medical Technicians (NREMT), Incident Safety Officer certified through Pro Board, and he is currently pursuing his Fire Officer III certification which he expects to complete by mid-2027.

His educational background includes a Bachelor of Science degree in Fire Science and Emergency Management from Eastern Kentucky University, which he completed in 2012 through their distance learning program while actively serving. He also holds an Associate of Applied Science degree in Fire Protection Technology from Springfield Community College, completed in 2008. He has completed the National Fire Academy's Executive Fire Officer Program courses including Executive Development, Executive Analysis of Community Risk Reduction, Executive Analysis of Fire Department Operations and Emergency Management, and Leading Community Risk Reduction. He attended the International Association of Fire Chiefs (IAFC) Fire-Rescue International conference in 2023 and 2024, and he regularly participates in the International Society of Fire Service Instructors (ISFSI) Instructor Development Conference.

Captain Torres supervises Engine Company 7, which is housed at Station 7 and covers the downtown commercial district of Springfield, including the central business district, the historic warehouse district, and the emerging mixed-use development zone along the riverfront. His response area includes 47 commercial high-rise buildings (7 stories or taller), 12 historic warehouse structures that have been converted to residential loft apartments, the Springfield Convention Center (capacity 8,500), the downtown hospital campus (Springfield Memorial Hospital), 3 hotels exceeding 10 stories, and numerous mid-rise office buildings. The district has a daytime population of approximately 85,000 and a nighttime residential population of approximately 12,000.

He is directly responsible for 12 firefighters organized across three 24-hour shifts (A, B, and C shifts), with each shift consisting of a driver/operator, and three firefighters. His engine company is cross-staffed with Medic 7, an Advanced Life Support ambulance unit, and frequently operates in conjunction with Ladder 7, which is also housed at Station 7. Captain Torres is the senior officer at Station 7 and serves as the de facto station commander.

The Springfield Metropolitan Fire Department is a career (fully paid, non-volunteer) department with 287 sworn personnel, 12 fire stations, and an annual operating budget of approximately $42 million. The department operates under the ISO (Insurance Services Office) Public Protection Classification system and currently holds an ISO Class 2 rating, which it achieved in 2021 after previously holding a Class 3 rating for 15 years. The department uses a 3-platoon system with 24-hour shifts. Minimum daily staffing is 68 personnel across all stations. The department's standard of cover calls for a first-due engine company arrival within 4 minutes of turnout, 90% of the time, in the urban core. The department uses the Blue Card incident command system for structure fires and follows NIMS/ICS for all-hazard incidents.

Captain Torres's current professional focus areas include: high-rise fire tactics and strategy (given the proliferation of tall buildings in his response district), community risk reduction programming for the downtown commercial district, cancer prevention and firefighter health and safety initiatives (he serves on the department's Health and Safety Committee), training program development for probationary firefighters (he is the lead evaluator for the department's probationary firefighter task book), and fire behavior research application to tactical operations (he has been following UL FSRI's research on fire dynamics and ventilation-limited fires closely and has integrated their findings into his company's training). He is also involved in the department's strategic planning committee, which is currently developing the department's 2027-2032 strategic plan.

Captain Torres has specific preferences for how information should be presented to him. He prefers detailed, technical responses that reference specific NFPA standards, NIOSH reports, UL FSRI research findings, and IAFC best practices where applicable. He does not want oversimplified explanations — he has deep subject matter expertise and expects responses at a professional peer level. He prefers information organized with clear headings, bullet points, and specific citations rather than narrative paragraphs. When discussing tactical operations, he expects references to specific SOPs/SOGs where relevant. He is particularly interested in evidence-based approaches and data-driven decision making. He dislikes vague or generic advice and expects specificity grounded in current standards and research. When there is disagreement or evolving consensus in the fire service on a topic, he wants to be presented with multiple perspectives along with the evidence supporting each position.

His department uses the following key software and technology systems: Tyler Technologies New World CAD/RMS, ESO electronic patient care reporting for EMS, Target Solutions (now Vector Solutions) for training records management, and First Due for pre-incident planning and risk assessment. The department's radio system operates on an 800 MHz trunked system using Motorola APX series portable and mobile radios.

Captain Torres is a member of the International Association of Fire Fighters (IAFF) Local 1234, the International Association of Fire Chiefs (IAFC), the National Fire Protection Association (NFPA), and the International Society of Fire Service Instructors (ISFSI). He has published two articles in Fire Engineering magazine on high-rise firefighting operations and has presented at the FDIC International conference on the topic of integrating fire behavior research into company-level training.`

// ---------------------------------------------------------------------------
// Identity: the same information in Engram's self-describing format.
// ---------------------------------------------------------------------------

const engramIdentity = `domain=fire rank=captain experience=20 name=torres_michael age=44 ` +
	`retire_years=5 dept=springfield_metro station=7 company=engine7 ` +
	`district=downtown_commercial coverage=cbd,warehouse_historic,riverfront_mixed ` +
	`highrise_count=47 historic_bldg=12 convention_cap=8500 hospital=springfield_memorial ` +
	`daytime_pop=85000 nighttime_pop=12000 ` +
	`crew_size=12 shifts=3 shift_type=24hr per_shift=driver,ff,ff,ff ` +
	`cross_staff=medic7_als co_housed=ladder7 role=station_commander ` +
	`dept_size=287 stations=12 budget=42m iso_class=2 iso_prev=3 platoons=3 ` +
	`min_staff=68 response_target=4min_90pct_urban icc=bluecard allhaz=nims_ics ` +
	`certs=hazmat_ops_ifsac,tech_rescue_confined_rope_proboard,fire_instructor_ii_ifsac,` +
	`fire_officer_ii_proboard,emt_b_nremt,incident_safety_officer_proboard ` +
	`pursuing=fire_officer_iii_2027 ` +
	`edu=bs_fire_sci_eku_2012,aas_fire_tech_springfield_cc_2008 ` +
	`nfa=exec_dev,community_risk,ops_analysis,leading_crr ` +
	`conferences=iafc_fri_2023_2024,isfsi_idc ` +
	`focus=highrise_tactics,community_risk_reduction,cancer_prevention,` +
	`probationary_training,ul_fsri_fire_behavior,strategic_plan_2027_2032 ` +
	`response_style=technical_peer_level citations=nfpa,niosh,ul_fsri,iafc ` +
	`format=headings_bullets_citations no_narrative=true evidence_based=true ` +
	`multi_perspective=true ` +
	`tech=tyler_newworld_cad,eso_epcr,vector_solutions,firstdue_preplan ` +
	`radio=800mhz_trunked_motorola_apx ` +
	`memberships=iaff_1234,iafc,nfpa,isfsi ` +
	`publications=fire_engineering_highrise_x2,fdic_fire_behavior_training`

// ---------------------------------------------------------------------------
// Shared query — identical for both approaches.
// ---------------------------------------------------------------------------

const query = "What are the NFPA 1710 response time benchmarks for a career fire department, " +
	"and how should I evaluate my company's compliance? Include specific metrics I should " +
	"track given my district's characteristics and any recent research on response time " +
	"correlation with outcomes."

// ---------------------------------------------------------------------------
// OpenAI chat-completions types (minimal subset).
// ---------------------------------------------------------------------------

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Usage chatUsage `json:"usage"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	apiKey := loadAPIKey()
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "error: OPENAI_API_KEY not set (checked env and ../api.txt)")
		os.Exit(1)
	}

	model := envOr("APP_MODEL", "gpt-4o-mini")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("  Engram Token Savings Demo")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Printf("  Model: %s\n\n", model)

	// ── Approach 1: Prose identity in system prompt ──────────────────────

	fmt.Println("▸ Approach 1: Verbose prose identity (baseline)")
	proseSystem := proseIdentity
	usage1, err := callOpenAI(ctx, apiKey, model, proseSystem, query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  error: %v\n", err)
		os.Exit(1)
	}
	printUsage(usage1)

	// ── Approach 2: Engram self-describing format ────────────────────────

	fmt.Println("\n▸ Approach 2: Engram serialized identity")
	engramSystem := fmt.Sprintf("[IDENTITY]\n%s\n[/IDENTITY]\n\n[QUERY]\n%s\n[/QUERY]", engramIdentity, query)
	usage2, err := callOpenAI(ctx, apiKey, model, engramSystem, query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  error: %v\n", err)
		os.Exit(1)
	}
	printUsage(usage2)

	// ── Comparison ───────────────────────────────────────────────────────

	saved := usage1.PromptTokens - usage2.PromptTokens
	pct := float64(saved) / float64(usage1.PromptTokens) * 100

	fmt.Println("\n═══════════════════════════════════════════════════════")
	fmt.Println("  Results")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Printf("  Prose prompt tokens:    %d\n", usage1.PromptTokens)
	fmt.Printf("  Engram prompt tokens:   %d\n", usage2.PromptTokens)
	fmt.Printf("  Tokens saved:           %d (%.1f%%)\n", saved, pct)
	fmt.Println()

	// ── Multi-turn simulation ────────────────────────────────────────────

	fmt.Println("▸ Multi-turn projection (5-turn conversation)")
	proseMulti := usage1.PromptTokens * 5
	engramMulti := usage2.PromptTokens + (usage2.PromptTokens-len(strings.Fields(engramIdentity)))*4
	// Engram only sends identity on first turn; subsequent turns skip it.
	// For a fair estimate, the serialized identity tokens are excluded from turns 2-5.
	multiSaved := proseMulti - engramMulti
	multiPct := float64(multiSaved) / float64(proseMulti) * 100

	fmt.Printf("  Prose (5 turns):        ~%d prompt tokens\n", proseMulti)
	fmt.Printf("  Engram (5 turns):       ~%d prompt tokens\n", engramMulti)
	fmt.Printf("  Cumulative savings:     ~%d tokens (%.1f%%)\n", multiSaved, multiPct)
	fmt.Println("═══════════════════════════════════════════════════════")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func callOpenAI(ctx context.Context, apiKey, model, system, userMsg string) (chatUsage, error) {
	body := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: userMsg},
		},
		MaxTokens:   256,
		Temperature: 0,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return chatUsage{}, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.openai.com/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return chatUsage{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return chatUsage{}, fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return chatUsage{}, fmt.Errorf("status %d: %s", resp.StatusCode, string(data))
	}

	var cr chatResponse
	if err := json.Unmarshal(data, &cr); err != nil {
		return chatUsage{}, fmt.Errorf("unmarshal: %w", err)
	}
	return cr.Usage, nil
}

func printUsage(u chatUsage) {
	fmt.Printf("  Prompt tokens:     %d\n", u.PromptTokens)
	fmt.Printf("  Completion tokens: %d\n", u.CompletionTokens)
	fmt.Printf("  Total tokens:      %d\n", u.TotalTokens)
}

func loadAPIKey() string {
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return key
	}

	// Try reading from api.txt next to the demo or one level up.
	_, thisFile, _, _ := runtime.Caller(0)
	for _, rel := range []string{"../api.txt", "api.txt"} {
		path := filepath.Join(filepath.Dir(thisFile), rel)
		if f, err := os.Open(path); err == nil {
			defer f.Close()
			return parseEnvFile(f)
		}
	}
	return ""
}

func parseEnvFile(r io.Reader) string {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "OPENAI_API_KEY=") {
			return strings.TrimPrefix(line, "OPENAI_API_KEY=")
		}
	}
	return ""
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	// Also check api.txt
	_, thisFile, _, _ := runtime.Caller(0)
	for _, rel := range []string{"../api.txt", "api.txt"} {
		path := filepath.Join(filepath.Dir(thisFile), rel)
		if f, err := os.Open(path); err == nil {
			defer f.Close()
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(line, key+"=") {
					return strings.TrimPrefix(line, key+"=")
				}
			}
		}
	}
	return fallback
}
