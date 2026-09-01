# Agent workflow

Repository work is coordinated through the file-backed board in `docs/tasks/`.
Before starting an implementation task:

1. Run `make tasks` and choose an unblocked `ready` card.
2. Claim it with `./scripts/kanban claim <id> <agent-name>` before editing code.
3. Work on only one `in_progress` card at a time.
4. Keep the card's acceptance criteria and verification commands current.
5. Use `./scripts/kanban move <id> blocked` when progress requires an external
   decision, and explain the blocker in the card's Notes section.
6. Run the card's verification and `./scripts/kanban check`, then use
   `./scripts/kanban move <id> done` when its outcome is complete.

Do not maintain a hand-written column index. Each card is an independent file
so parallel work normally touches different files; `./scripts/kanban list`
derives the current columns. Keep the planning context in the card current when
implementation changes what is known. See `docs/tasks/README.md` for card and
lifecycle conventions.
