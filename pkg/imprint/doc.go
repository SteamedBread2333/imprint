// Package imprint is the portable markdown vault for long-lived agent preferences.
//
// Rules pack into imprint-NNNN.md shards as concatenated YAML-frontmatter
// documents. A new shard starts at DefaultMaxShardLines (32768) or
// DefaultMaxShardBytes (1 MiB). Legacy one-file-per-rule r-YYYY-MM-DD-NNN.md
// is still read and compacted on open. IDs look like r-YYYY-MM-DD-NNN.
// The vault is a directory (default ./memory) plus an archive/ subdirectory
// for superseded and dormant rules. Find ranks with scope AND then BM25.
// Get fills ReferencedBy. Forget strips inbound relationship ids.
package imprint
