// Mermaid diagram definitions for the Pipeline page. These mirror docs/AI_PIPELINE.md
// (kept in sync manually).

export const systemArchitecture = `flowchart LR
    GH["GitHub"] -->|"PR webhook"| WH["Webhook handler<br/>(HMAC verify + dedup)"]
    WH -->|"enqueue"| Q["Durable job queue<br/>(Postgres, 2 workers)"]
    Q --> ENG["Review Engine<br/>ProcessPullRequest"]
    ENG <-->|"stdio JSON-RPC"| MCP["GitHub MCP server<br/>(get PR, files, diff, post review)"]
    MCP <--> GH
    ENG <-->|"chat / tool calls"| LLM["LLM provider<br/>(Groq / OpenAI / Anthropic)"]
    ENG <-->|"index / retrieve"| RAG["RAG: Qdrant + embeddings<br/>(optional)"]
    ENG -->|"persist review, comments,<br/>trace, cost"| DB[("PostgreSQL")]
    DB --> API["REST API (Gin)"]
    API --> FE["React dashboard<br/>(reviews, cost, agent trace, live feed)"]`;

export const reviewPipeline = `flowchart TD
    START(["Job: owner/repo PR"]) --> LOOKUP["Repo lookup + auto-review gate"]
    LOOKUP -->|"inactive / off"| STOP(["Skip"])
    LOOKUP --> FETCH["Fetch PR + changed files + diff via MCP"]
    FETCH --> REC["Create review record"]
    REC --> BUILD["Build file contexts<br/>exclude patterns, skip lockfiles,<br/>truncate patch to 2000 chars"]
    BUILD --> SORT["SortByPriority<br/>(security / high-churn first)"]
    SORT --> P1{"Phase 1 - Triage<br/>files >= 4?"}
    P1 -->|"no (small PR)"| ALL["Review all files"]
    P1 -->|"yes"| TRIAGE["LLM triage (triage model)<br/>pick up to 8 of up to 20 files"]
    TRIAGE --> RAGIDX
    ALL --> RAGIDX
    RAGIDX{"RAG_ENABLED?"} -->|"yes"| IDX["Index changed files to Qdrant<br/>(best-effort)"]
    RAGIDX -->|"no"| P2
    IDX --> P2
    P2{"Phase 2 - Review<br/>model supports tools?"}
    P2 -->|"yes"| AGENT["Run AGENT tool-use loop"]
    P2 -->|"no / agent errored"| FIXED["Fixed single-shot review (JSON)"]
    AGENT -->|"agent error"| FIXED
    AGENT --> FINDINGS["Findings + summary"]
    FIXED --> FINDINGS
    FINDINGS --> POST["AdjustSeverities -> Deduplicate -><br/>remove already-published"]
    POST --> P3["Phase 3 - Reflection (reflection model)<br/>drop confidence &lt; 0.7 or invalid"]
    P3 --> PUB["Publish via MCP<br/>REQUEST_CHANGES if critical, else COMMENT"]
    PUB --> STORE["Store comments + tokens + USD cost"]
    STORE --> DONE(["Completed"])`;

export const agentLoop = `flowchart TD
    A(["Agent start"]) --> SYS["System prompt + initial context<br/>(PR meta + changed-file diffs)"]
    SYS --> STEP{"step <= maxSteps (8)?"}
    STEP -->|"no"| BUDGET(["Give up -> fixed-pipeline fallback"])
    STEP -->|"yes"| CALL["llm.ChatWithTools(messages, tools)"]
    CALL --> HASTOOL{"tool calls returned?"}
    HASTOOL -->|"no + parses as review"| FIN(["Return findings"])
    HASTOOL -->|"no + unparseable"| BUDGET
    HASTOOL -->|"yes"| DEDUP{"identical call already made?"}
    DEDUP -->|"yes"| NOTE["Return 'already called' note"] --> APPEND
    DEDUP -->|"no"| WHICH{"which tool?"}
    WHICH -->|"get_file_diff"| T1["Read diff from PR patch map"]
    WHICH -->|"get_file_contents"| T2["MCP get_file_contents (@ head)"]
    WHICH -->|"search_code"| T3["MCP search_code(query)"]
    WHICH -->|"retrieve_context = RAG"| T4["rag.Retrieve(repo, query, k=5)<br/>embed query -> Qdrant top-k"]
    WHICH -->|"submit_findings (terminal)"| FIN
    T1 --> APPEND["Append tool result to messages<br/>(record trace step)"]
    T2 --> APPEND
    T3 --> APPEND
    T4 --> APPEND
    APPEND --> STEP`;

export const ragFlow = `flowchart LR
    subgraph Indexing ["Indexing (before review, best-effort)"]
      CF["Changed files<br/>(full contents via MCP)"] --> CH["Chunk<br/>60-line windows, 10 overlap"]
      CH --> EM1["Embed<br/>(Ollama, nomic-embed-text, dim 768)"]
      EM1 --> UP["Upsert points to Qdrant<br/>stable id = uuid(repo|path|startLine)"]
    end
    subgraph Retrieval ["Retrieval (agent retrieve_context tool)"]
      QRY["query"] --> EM2["Embed query"]
      EM2 --> SEARCH["Qdrant search<br/>filter: repo, top-k = 5, cosine"]
      SEARCH --> CHUNKS["Ranked code chunks<br/>(path + line range + content)"]
      CHUNKS --> BACK["back to the agent as tool result"]
    end`;

export const reviewSequence = `sequenceDiagram
    autonumber
    participant GH as GitHub
    participant WH as Webhook
    participant Q as Job Queue
    participant E as Engine
    participant MCP as GitHub MCP
    participant L as LLM
    participant R as RAG Qdrant
    participant DB as Postgres
    GH->>WH: PR opened/updated (signed)
    WH->>Q: enqueue (dedup by delivery id)
    Q->>E: ProcessPullRequest
    E->>MCP: get PR + changed files + diff
    MCP->>GH: GitHub API
    E->>DB: create review
    E->>L: Phase 1 - triage
    opt RAG_ENABLED
        E->>MCP: fetch full file contents
        E->>R: chunk + embed + upsert
    end
    E->>L: Phase 2 - agent ChatWithTools
    loop until submit_findings / maxSteps
        L-->>E: tool call
        alt get_file_contents / search_code
            E->>MCP: fetch / search
        else retrieve_context
            E->>R: embed + top-k search
        else get_file_diff
            E->>E: read PR patch map
        end
        E-->>L: tool result
    end
    L-->>E: submit_findings
    E->>L: Phase 3 - reflection
    E->>MCP: publish review
    MCP->>GH: post inline comments
    E->>DB: store comments + tokens + cost`;

export interface PipelineDiagram {
  title: string;
  description: string;
  chart: string;
}

export const pipelineDiagrams: PipelineDiagram[] = [
  {
    title: 'System architecture',
    description: 'How a GitHub PR event flows through the stack to posted review comments.',
    chart: systemArchitecture,
  },
  {
    title: 'End-to-end review pipeline',
    description:
      'The three-phase pipeline with every decision gate: triage, RAG indexing, the agent (with fallback), reflection, and publish.',
    chart: reviewPipeline,
  },
  {
    title: 'Agent tool-use loop (Phase 2)',
    description:
      'The agent decides which tools to call — including retrieve_context (RAG) — before submitting findings. Bounded by maxSteps with call dedup.',
    chart: agentLoop,
  },
  {
    title: 'RAG: index & retrieve',
    description: 'Only when RAG is enabled: chunk + embed + upsert to Qdrant, then semantic top-k retrieval for the agent.',
    chart: ragFlow,
  },
  {
    title: 'Sequence: one full review',
    description: 'A full review across GitHub, MCP, LLM, RAG and Postgres, including the tool-call loop.',
    chart: reviewSequence,
  },
];
