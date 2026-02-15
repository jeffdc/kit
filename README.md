# kit

A collection of small, independent CLI tools. Each tool lives in its own directory with its own language, dependencies, and build system. They don't share code.

All of these are built and optimized to be driven by LLM agents. They are also almost entirely built by LLM agents. They are very opinionated towards my needs.

## Tools

### [watchmen](watchmen/)

Time tracking and invoice generation. Start a timer, do your work, stop the timer. Generate invoices from tracked time. Built in Go.

### [canal](canal/)

Inter-agent messaging system. Coordinates work between LLM agents -- planners dispatch work, workers execute and report back, humans get notified only when decisions are needed. Built in Elixir/Phoenix.

### [mull](mull/)

Idea and feature tracking for solo projects. Captures matters (ideas, features, tasks) as markdown files alongside your code. Designed for conversational use with AI coding assistants. Built in Go.
