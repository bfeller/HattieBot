# HattieBot Self-Improvement Flows

These flowcharts illustrate how the agent extends its own capabilities.

---

## 1. Tool Creation Flow

When the agent needs a capability it doesn't have, it can create a new tool.

```mermaid
graph TD
    Need([Agent Identifies Need]) --> Spawn[Spawn tool_creation Sub-Mind]
    Spawn --> Autohand[Call autohand CLI]
    Autohand --> Write[Write Go Code]
    Write --> Save[Save to $CONFIG_DIR/tools/]
    Save --> Build[Build Binary]
    Build --> Register[Call register_tool]
    Register --> Update[Update tools_registry DB]
    Update --> Available([Tool Now Available])
```

**Key Steps:**
1. Agent spawns `tool_creation` sub-mind (isolated context).
2. Sub-mind uses `autohand` CLI for complex code generation.
3. Builds binary with `go build` to `$CONFIG_DIR/bin`.
4. Registers via `register_tool`.
5. Tool becomes available for future calls.

---

## 2. Sub-Mind Creation Flow

When the agent needs a specialized reasoning mode.

```mermaid
graph TD
    Need([Agent Needs Specialized Mode]) --> Define[Define Sub-Mind Config]
    Define --> Save[Save to .hattiebot/subminds.json]
    Save --> Tools[Specify Allowed Tools]
    Tools --> Prompt[Write System Prompt]
    Prompt --> Available([Sub-Mind Now Available])
    
    Use([Later: Agent Invokes]) --> Spawn[spawn_submind tool]
    Spawn --> Isolated[Isolated Context Created]
    Isolated --> Execute[Sub-Mind Executes Task]
    Execute --> Return[Returns Result to Main Loop]
```

**Key Steps:**
1. Agent defines new mode in `subminds.json` via `manage_submind`.
2. Specifies: allowed tools, system prompt, limits.
3. Uses `spawn_submind` to invoke it later.

---

## 3. LLM Provider Creation Flow

When the agent needs to use a new LLM provider.

```mermaid
graph TD
    Need([Agent Needs New Provider]) --> Simple[Simple REST API?]
    
    Simple --> Template[Create JSON Template]
    Template --> SaveT[Save to .hattiebot/providers/]
    SaveT --> RegisterT[Update llm_routing.json]
    RegisterT --> Ready([Provider Ready])
    
    Complex([Complex Logic Needed]) --> Autohand[Use autohand CLI]
    Autohand --> Code[Write Go Provider Code]
    Code --> Build[Build Binary]
    Build --> SaveB[Save to .hattiebot/providers/]
    SaveB --> RegisterB[Register as type: binary]
    RegisterB --> Ready
```

**Key Steps:**
1. **Template path**: Write JSON spec for simple APIs via `manage_llm_provider`.
2. **Routing**: Use `manage_llm_provider` to update routing rules.

---

## 4. Self-Improvement Decision Tree

How the agent decides what to extend.

```mermaid
graph TD
    Task([User Request]) --> Can[Can Handle?]
    Can --> Execute[Execute Normally]
    
    Blocked([Cannot Handle]) --> Type[What's Missing?]
    Type --> CreateTool[Create Tool via autohand]
    Type --> CreateMind[Create Sub-Mind]
    Type --> CreateProvider[Create Provider]
    Type --> Learn[Store as Fact/Memory]
    
    CreateTool --> Retry[Retry Task]
    CreateMind --> Retry
    CreateProvider --> Retry
    Learn --> Retry
    Retry --> Execute
```

**Key Insight**: When blocked, the agent identifies the gap, uses `autohand` for complex code generation, creates the component, and retries.

---

## 5. Autohand Integration

`autohand` is an external AI coding CLI that handles complex workspace operations.

```mermaid
graph TD
    Agent([Agent Loop]) --> Need[Needs Complex Code]
    Need --> Tool[autohand_cli Tool]
    Tool --> CLI[autohand -p instruction]
    CLI --> Workspace[Modifies Workspace Files]
    Workspace --> Result[Returns stdout/stderr]
    Result --> Agent
```

**Usage**: Agent sends instruction string, autohand executes in workspace, returns result. Used for tool creation, provider plugins, and complex refactoring.
