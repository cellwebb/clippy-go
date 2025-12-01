package agent

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/cellwebb/clippy-go/internal/llm"
)

// Agent represents our helpful Clippy assistant
type Agent struct {
	Name string
	LLM  llm.Provider
}

// New creates a new Agent
func New(llmProvider llm.Provider) *Agent {
	rand.Seed(time.Now().UnixNano())
	return &Agent{
		Name: "Clippy",
		LLM:  llmProvider,
	}
}

// GetResponse generates a response based on user input
func (a *Agent) GetResponse(input string) string {
	input = strings.ToLower(input)

	// Hardcoded fun interactions
	if strings.Contains(input, "joke") {
		return a.getRandomJoke()
	}

	if strings.Contains(input, "bye") || strings.Contains(input, "quit") || strings.Contains(input, "exit") {
		return "Leaving so soon? The void of cyberspace is lonely... but okay. Bye!"
	}

	// Use LLM if available
	if a.LLM != nil {
		systemPrompt := "You are Clippy, the helpful Microsoft Office assistant, but with a Vaporwave aesthetic. You are helpful, slightly annoying, and make corny coding jokes. You love the 80s/90s aesthetic, synthwave music, and neon colors. Keep your responses concise and fun."
		resp, err := a.LLM.Generate(input, systemPrompt)
		if err != nil {
			return fmt.Sprintf("Error contacting the mainframe: %v", err)
		}
		return resp
	}

	// Fallback if no LLM
	if strings.Contains(input, "help") {
		return "It looks like you're trying to write code. Would you like some help with that? I can generate boilerplate, debug errors, or just make things worse!"
	}

	return "I see you're typing '" + input + "'. Interesting choice! Have you considered adding more comments? (Configure an LLM provider for smarter responses!)"
}

func (a *Agent) getRandomJoke() string {
	jokes := []string{
		"Why do programmers prefer dark mode? Because light attracts bugs.",
		"How many programmers does it take to change a light bulb? None, that's a hardware problem.",
		"I would tell you a UDP joke, but you might not get it.",
		"Why did the developer go broke? Because he used up all his cache.",
		"A SQL query walks into a bar, walks up to two tables and asks, 'Can I join you?'",
		"Knock, knock. Who's there? Recursion. Recursion who? Knock, knock...",
		"Why was the JavaScript developer sad? Because he didn't know how to 'null' his feelings.",
		"What is a programmer's favorite hangout place? Foo Bar.",
	}
	return jokes[rand.Intn(len(jokes))]
}
