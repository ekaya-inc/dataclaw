# CONSIDER: Semantic / RAG / Hybrid Search Subsystem

pgEdge-style embedding generation, BM25+MMR hybrid search, vector schema discovery, and semantic-search setup — adds embedding providers, chunking, ranking, credentials, and engine-specific vector extensions.

Deferred: provider/credential ownership, cross-engine parity (vector features differ), and scope creep beyond safe SQL/MCP tooling.

If revisited: start with progressive output modes (`ids_only`, `summary`, `full`) and adapter capability probes. Cross-referenced from TODO "Future Search Tools".

Reference: https://github.com/pgEdge/pgedge-postgres-mcp
