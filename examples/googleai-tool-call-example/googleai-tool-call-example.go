package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/googleai"
)

func crypto(c string) string {
	// Same as your curl URL
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/simple/price?ids=%s&vs_currencies=usd", c)

	// Make HTTP request
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	// Parse JSON into a generic map
	var result map[string]map[string]float64
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		panic(err)
	}

	// Extract Bitcoin USD price
	price := result["bitcoin"]["usd"]
	s := fmt.Sprintf("Current Bitcoin price: $%.2f\n", price)
	return s
}

func main() {
	genaiKey := os.Getenv("GOOGLE_API_KEY")
	if genaiKey == "" {
		log.Fatal("please set GOOGLE_API_KEY")
	}

	ctx := context.Background()

	llm, err := googleai.New(ctx, googleai.WithAPIKey(genaiKey))
	if err != nil {
		log.Fatal(err)
	}

	// Start by sending an initial question about the weather to the model, adding
	// "available tools" that include a getCurrentWeather function.
	// Thoroughout this sample, messageHistory collects the conversation history
	// with the model - this context is needed to ensure tool calling works
	// properly.
	messageHistory := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "What is the current value of ethereum ?"),
	}
	resp, err := llm.GenerateContent(ctx, messageHistory, llms.WithTools(availableTools))
	if err != nil {
		log.Fatal(err)
	}

	// Translate the model's response into a MessageContent element that can be
	// added to messageHistory.
	respchoice := resp.Choices[0]
	assistantResponse := llms.TextParts(llms.ChatMessageTypeAI, respchoice.Content)
	for _, tc := range respchoice.ToolCalls {
		assistantResponse.Parts = append(assistantResponse.Parts, tc)
	}
	messageHistory = append(messageHistory, assistantResponse)

	// "Execute" tool calls by calling requested function
	for _, tc := range respchoice.ToolCalls {
		switch tc.FunctionCall.Name {
		case "getCrypto":
			log.Println("getCrypto...")
			log.Println("Arguments " + tc.FunctionCall.Arguments)

			var args struct {
				Crypto string `json:"crypto"`
			}
			if err := json.Unmarshal([]byte(tc.FunctionCall.Arguments), &args); err != nil {
				log.Fatal(err)
			}
			s := crypto(args.Crypto)
			toolResponse := llms.MessageContent{
				Role: llms.ChatMessageTypeTool,
				Parts: []llms.ContentPart{
					llms.ToolCallResponse{
						Name:    tc.FunctionCall.Name,
						Content: s,
					},
				},
			}
			messageHistory = append(messageHistory, toolResponse)
		default:
			log.Fatalf("got unexpected function call: %v", tc.FunctionCall.Name)
		}
	}

	resp, err = llm.GenerateContent(ctx, messageHistory, llms.WithTools(availableTools))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Response after tool call:")
	b, _ := json.MarshalIndent(resp.Choices[0], " ", "  ")
	fmt.Println(string(b))
}

// availableTools simulates the tools/functions we're making available for
// the model.
var availableTools = []llms.Tool{
	{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        "getCrypto",
			Description: "Get the current value of a particular cryptocurrency",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"crypto": map[string]any{
						"type":        "string",
						"description": "The id of a cryto currency",
					},
				},
				"required": []string{"crypto"},
			},
		},
	},
}
