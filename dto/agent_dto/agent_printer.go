package agent_dto

type AgentPrinter interface {
	PrintForAgent() (string, error)
}
