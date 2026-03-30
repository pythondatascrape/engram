// demo/travel/main.go — Travel app demo: prose vs Engram identity for trip planning.
//
// Usage:
//
//	go run ./demo/travel
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
// Prose identity: what a travel app would stuff into the system prompt today.
// ---------------------------------------------------------------------------

const proseIdentity = `You are a travel planning assistant for Sarah Chen. Here is everything you need to know about Sarah to give her personalized travel recommendations:

Sarah is a 32-year-old woman who lives in Austin, Texas. She is planning a trip with her partner (2 people total, no children, no pets). They have a moderate budget — they're comfortable spending money on good experiences but aren't looking for five-star luxury. They typically spend around $150-200 per night on lodging and $50-75 per person per day on food and activities.

Their travel style is best described as a mix of foodie and cultural exploration. They love discovering local restaurants, food markets, street food, breweries, and regional cuisine. They also enjoy visiting museums, historic neighborhoods, art galleries, live music venues, and cultural landmarks. They are not particularly interested in extreme outdoor activities like hiking or rock climbing, but they enjoy casual outdoor experiences like walking through parks, visiting botanical gardens, or strolling along waterfronts. On a scale of 1-5, their outdoor interest is about a 2. They enjoy moderate nightlife — a good cocktail bar or live music venue, but not clubbing. Nightlife interest is about a 3. History interest is about a 4.

For lodging, they strongly prefer boutique hotels or well-reviewed Airbnb apartments in walkable neighborhoods close to restaurants and attractions. They do not want chain hotels, resorts, or anything outside the city center. They want to be able to walk to most things.

For transportation, they prefer to fly to their destination and then use a combination of walking, public transit, rideshare, and occasional rental cars. They do not want to do a road trip or drive long distances. They own a car but prefer not to use it on vacation.

Their travel pace is moderate — they don't want every minute scheduled, but they also don't want to sit around with nothing to do. They like having 2-3 planned activities or meals per day with free time in between for spontaneous exploration. They hate feeling rushed.

Sarah has traveled moderately within the US — she has visited most major cities on the coasts (New York, Los Angeles, San Francisco, Chicago, Seattle, Miami, Boston) but has less experience with the Southeast, Midwest, and Mountain West regions. She is interested in exploring places she hasn't been.

They are planning a trip for the fall season, specifically October, and have about 5-7 days available. Sarah prefers fall travel because she enjoys cooler weather and fall foliage. She specifically wants to avoid extreme heat and large tourist crowds.

Sarah is a vegetarian but her partner is not — she needs restaurants that have good vegetarian options but aren't exclusively vegetarian, since her partner eats everything. She has no other dietary restrictions or food allergies.

Sarah has no mobility limitations. She can walk long distances and handle stairs, hills, etc. without any issues.

When making recommendations, please consider all of these preferences. Suggest specific restaurants, neighborhoods, and activities that match their interests. Be specific with names and locations rather than giving generic advice. If you recommend a city or region, explain why it's a good fit for their specific preferences. When discussing food, always note vegetarian options. When discussing lodging, suggest specific neighborhoods that are walkable and have good food scenes nearby.

Sarah prefers responses organized with clear sections for different aspects of the trip (where to stay, what to eat, what to do, how to get around). She likes having a rough daily itinerary framework but not a rigid hour-by-hour schedule. Include estimated costs where possible.`

// ---------------------------------------------------------------------------
// Engram serialized identity: same information, self-describing format.
// ---------------------------------------------------------------------------

const engramIdentity = `budget=moderate lodging_budget=150-200 food_budget=50-75pp ` +
	`travel_style=foodie,cultural group_size=2 has_kids=false has_pets=false ` +
	`age_range=26-35 mobility=full ` +
	`lodging=boutique,airbnb lodging_pref=walkable,city_center lodging_avoid=chains,resorts ` +
	`transport=fly local_transport=walk,transit,rideshare avoid_driving=true ` +
	`food_pref=vegetarian partner_food=anything ` +
	`pace=moderate activities_per_day=2-3 ` +
	`outdoors=2 nightlife=3 history_interest=4 ` +
	`home_state=TX home_city=austin ` +
	`visited=NYC,LA,SF,CHI,SEA,MIA,BOS ` +
	`region_pref=southeast,midwest,mountain ` +
	`trip_days=5-7 season=fall month=october ` +
	`avoid=heat,crowds ` +
	`format=sections,daily_framework,costs response_style=specific_names,neighborhoods`

// ---------------------------------------------------------------------------
// Queries — simulate a 5-turn travel planning conversation.
// ---------------------------------------------------------------------------

var queries = []string{
	"What are the best cities for a fall trip that match my preferences? Give me your top 3 picks with reasons.",
	"I'm leaning toward Asheville. Build me a 5-day itinerary with specific restaurant and neighborhood recommendations.",
	"What's the best neighborhood to stay in and can you suggest 3 specific boutique hotels or Airbnbs?",
	"What are the must-try restaurants for a vegetarian with a non-vegetarian partner? I want breakfast, lunch, and dinner spots.",
	"What day trips are worth doing from Asheville? I want fall foliage and maybe a small town to explore.",
}

// ---------------------------------------------------------------------------
// OpenAI types
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
		fmt.Fprintln(os.Stderr, "error: OPENAI_API_KEY not set")
		os.Exit(1)
	}
	model := envOr("APP_MODEL", "gpt-4o-mini")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("  Engram Travel App Demo — 5-Turn Conversation")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("  Model: %s\n", model)
	fmt.Printf("  Turns: %d\n\n", len(queries))

	// ── Run both approaches across all 5 turns ──────────────────────────

	var proseHistory, engramHistory []chatMessage
	var proseTotalPrompt, engramTotalPrompt int
	var proseTotalCompletion, engramTotalCompletion int

	for i, q := range queries {
		fmt.Printf("──── Turn %d ────────────────────────────────────────────\n", i+1)
		fmt.Printf("  Q: %s\n\n", truncate(q, 80))

		// -- Prose approach: identity re-sent as system prompt every turn --
		proseMessages := []chatMessage{{Role: "system", Content: proseIdentity}}
		proseMessages = append(proseMessages, proseHistory...)
		proseMessages = append(proseMessages, chatMessage{Role: "user", Content: q})

		u1, reply1, err := callOpenAI(ctx, apiKey, model, proseMessages)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  prose error: %v\n", err)
			os.Exit(1)
		}
		proseTotalPrompt += u1.PromptTokens
		proseTotalCompletion += u1.CompletionTokens
		proseHistory = append(proseHistory, chatMessage{Role: "user", Content: q})
		proseHistory = append(proseHistory, chatMessage{Role: "assistant", Content: reply1})

		// -- Engram approach: serialized identity only on turn 1 --
		var engramMessages []chatMessage
		if i == 0 {
			sys := fmt.Sprintf("[IDENTITY]\n%s\n[/IDENTITY]", engramIdentity)
			engramMessages = []chatMessage{{Role: "system", Content: sys}}
		} else {
			engramMessages = []chatMessage{{Role: "system", Content: "Continue assisting the traveler from the identity provided in turn 1."}}
		}
		engramMessages = append(engramMessages, engramHistory...)
		engramMessages = append(engramMessages, chatMessage{Role: "user", Content: q})

		u2, reply2, err := callOpenAI(ctx, apiKey, model, engramMessages)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  engram error: %v\n", err)
			os.Exit(1)
		}
		engramTotalPrompt += u2.PromptTokens
		engramTotalCompletion += u2.CompletionTokens
		engramHistory = append(engramHistory, chatMessage{Role: "user", Content: q})
		engramHistory = append(engramHistory, chatMessage{Role: "assistant", Content: reply2})

		fmt.Printf("  %-12s Prompt: %4d  Completion: %4d  Total: %4d\n",
			"Prose:", u1.PromptTokens, u1.CompletionTokens, u1.TotalTokens)
		fmt.Printf("  %-12s Prompt: %4d  Completion: %4d  Total: %4d\n",
			"Engram:", u2.PromptTokens, u2.CompletionTokens, u2.TotalTokens)

		saved := u1.PromptTokens - u2.PromptTokens
		pct := float64(saved) / float64(u1.PromptTokens) * 100
		fmt.Printf("  Saved:       %d prompt tokens (%.1f%%)\n\n", saved, pct)

		// Small delay to avoid rate limiting.
		time.Sleep(500 * time.Millisecond)
	}

	// ── Final summary ────────────────────────────────────────────────────

	totalSaved := proseTotalPrompt - engramTotalPrompt
	totalPct := float64(totalSaved) / float64(proseTotalPrompt) * 100

	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("  Cumulative Results (5 Turns)")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("  Prose total prompt tokens:    %d\n", proseTotalPrompt)
	fmt.Printf("  Engram total prompt tokens:   %d\n", engramTotalPrompt)
	fmt.Printf("  Total tokens saved:           %d (%.1f%%)\n", totalSaved, totalPct)
	fmt.Println()
	fmt.Printf("  Prose total (prompt+comp):    %d\n", proseTotalPrompt+proseTotalCompletion)
	fmt.Printf("  Engram total (prompt+comp):   %d\n", engramTotalPrompt+engramTotalCompletion)
	fmt.Println()

	// Cost estimate (gpt-4o-mini: $0.15/1M input, $0.60/1M output)
	proseCost := float64(proseTotalPrompt)*0.15/1e6 + float64(proseTotalCompletion)*0.60/1e6
	engramCost := float64(engramTotalPrompt)*0.15/1e6 + float64(engramTotalCompletion)*0.60/1e6
	fmt.Printf("  Estimated cost (gpt-4o-mini):\n")
	fmt.Printf("    Prose:  $%.6f\n", proseCost)
	fmt.Printf("    Engram: $%.6f\n", engramCost)
	fmt.Printf("    Saved:  $%.6f (%.1f%%)\n", proseCost-engramCost,
		(proseCost-engramCost)/proseCost*100)
	fmt.Println()

	// Scale projection
	fmt.Println("▸ At scale: 10,000 users × 5 turns/session × 30 sessions/month")
	monthlyProse := float64(proseTotalPrompt) * 10000 * 30
	monthlyEngram := float64(engramTotalPrompt) * 10000 * 30
	monthlySaved := monthlyProse - monthlyEngram
	monthlyProseCost := monthlyProse*0.15/1e6 + float64(proseTotalCompletion)*10000*30*0.60/1e6
	monthlyEngramCost := monthlyEngram*0.15/1e6 + float64(engramTotalCompletion)*10000*30*0.60/1e6

	fmt.Printf("  Monthly prompt tokens saved: %.0fM\n", monthlySaved/1e6)
	fmt.Printf("  Monthly cost: Prose $%.2f → Engram $%.2f\n", monthlyProseCost, monthlyEngramCost)
	fmt.Printf("  Monthly savings: $%.2f\n", monthlyProseCost-monthlyEngramCost)
	fmt.Println("═══════════════════════════════════════════════════════════")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func callOpenAI(ctx context.Context, apiKey, model string, messages []chatMessage) (chatUsage, string, error) {
	body := chatRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   512,
		Temperature: 0,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return chatUsage{}, "", fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.openai.com/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return chatUsage{}, "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return chatUsage{}, "", fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return chatUsage{}, "", fmt.Errorf("status %d: %s", resp.StatusCode, string(data))
	}

	var cr struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage chatUsage `json:"usage"`
	}
	if err := json.Unmarshal(data, &cr); err != nil {
		return chatUsage{}, "", fmt.Errorf("unmarshal: %w", err)
	}

	reply := ""
	if len(cr.Choices) > 0 {
		reply = cr.Choices[0].Message.Content
	}
	return cr.Usage, reply, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func loadAPIKey() string {
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return key
	}
	_, thisFile, _, _ := runtime.Caller(0)
	for _, rel := range []string{"../../api.txt", "../api.txt", "api.txt"} {
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
	_, thisFile, _, _ := runtime.Caller(0)
	for _, rel := range []string{"../../api.txt", "../api.txt", "api.txt"} {
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
