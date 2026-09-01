# Planning and task board

Planning is a first-class repository artifact. This board answers “what is
next?”, preserves why work is worth doing, and makes enough context explicit
for people and agents to execute tasks independently.

The Markdown cards in `cards/` are the source of truth. There is deliberately no
hand-maintained column index: `make tasks` derives the kanban view from card
metadata, so claiming two unrelated tasks changes two unrelated files.

## Workflow

The columns are:

- `backlog`: valuable work that still needs shaping, ordering, or a dependency;
- `ready`: a small, independently actionable outcome with acceptance criteria
  and a verification plan;
- `in_progress`: claimed by exactly one agent;
- `blocked`: started work waiting on the decision or condition named in Notes;
- `done`: the planned outcome and its verification are complete.

Use the board through the repository script:

```sh
make tasks
./scripts/kanban next
./scripts/kanban claim T-001 agent-name
./scripts/kanban move T-001 blocked
./scripts/kanban move T-001 ready
./scripts/kanban move T-001 done
./scripts/kanban check
```

Claims are serialized in a shared checkout and reject a second active card for
the same owner. Across separate clones or branches, Git cannot provide a global
lock: push the claim before implementation, pull before choosing a card, and
resolve competing claims in favor of the first claim merged to the shared
branch.

## Planning rules

- One Markdown card is one independently mergeable outcome.
- Explain the motivation, current evidence, scope boundaries, and open questions
  before promoting a card from `backlog` to `ready`.
- IDs are stable and filenames begin with the ID.
- `priority` is `0` (highest) through `4` (lowest).
- `owner` is `unassigned` unless the card is `in_progress` or `blocked`.
- `depends_on` is `none` or a comma-separated list of card IDs.
- A `ready` card has falsifiable acceptance criteria and verification commands
  or a concrete manual verification procedure.
- Discovery is part of planning: update the current card when it changes the
  intended outcome, and create a separate card for adjacent work.
- Cards can propose interfaces or implementation approaches, but must label
  unsettled choices as open questions. An accepted costly-to-reverse choice
  still gets an ADR.
- When a task is done, update durable code, tests, runbooks, references, ADRs,
  and changelog entries as appropriate. Keep the completed card through the
  current milestone review; Git history remains the long-term archive after it
  is pruned.

Copy [TEMPLATE.md](TEMPLATE.md) into `cards/`, allocate the next unused numeric
ID, and run `./scripts/kanban check`.
