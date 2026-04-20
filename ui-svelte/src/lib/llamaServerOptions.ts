export type OptionCategory =
  | "General"
  | "Runtime"
  | "Memory"
  | "Model"
  | "Sampling"
  | "Server"
  | "Embeddings"
  | "Chat"
  | "Speculative"
  | "Logging"
  | "Advanced";

export type OptionValueMode = "none" | "required" | "optional";

export interface LlamaServerOption {
  id: string;
  aliases: string[];
  label: string;
  category: OptionCategory;
  valueMode: OptionValueMode;
  valueCount?: number;
  explanation: string;
}

export interface ParsedOption {
  option: LlamaServerOption;
  flag: string;
  value: string | null;
  raw: string;
  enabled: boolean | null;
}

export interface UnknownOption {
  flag: string;
  value: string | null;
  raw: string;
}

export interface ModelConfigInterpretation {
  provider: "llama.cpp" | "unknown";
  executable: string;
  args: string[];
  options: ParsedOption[];
  unknownOptions: UnknownOption[];
  positionals: string[];
  highlights: ConfigHighlight[];
  categories: Array<{ category: OptionCategory; options: ParsedOption[] }>;
}

export interface ConfigHighlight {
  id: string;
  label: string;
  value: string;
  note: string;
  tone: "neutral" | "good" | "warning";
}

const source = "llama.cpp tools/server/README.md";

export const LLAMA_SERVER_OPTION_SOURCE =
  "https://github.com/ggml-org/llama.cpp/blob/master/tools/server/README.md";

const defs = <T extends LlamaServerOption[]>(items: T): T => items;

export const LLAMA_SERVER_OPTIONS = defs([
  opt("help", ["-h", "--help", "--usage"], "Help", "General", "none", "Prints command usage and exits."),
  opt("version", ["--version"], "Version", "General", "none", "Prints llama.cpp version and build data."),
  opt("license", ["--license"], "License", "General", "none", "Prints source license and dependency license information."),
  opt("cache-list", ["-cl", "--cache-list"], "Cache list", "General", "none", "Lists models already available in the llama.cpp cache."),
  opt("completion-bash", ["--completion-bash"], "Bash completion", "General", "none", "Outputs shell completion for llama.cpp commands."),

  opt("threads", ["-t", "--threads"], "Generation threads", "Runtime", "required", "CPU threads used while generating tokens."),
  opt("threads-batch", ["-tb", "--threads-batch"], "Batch threads", "Runtime", "required", "CPU threads used for prompt and batch processing."),
  opt("cpu-mask", ["-C", "--cpu-mask"], "CPU mask", "Runtime", "required", "Pins generation work to CPUs selected by a hexadecimal affinity mask."),
  opt("cpu-range", ["-Cr", "--cpu-range"], "CPU range", "Runtime", "required", "Pins generation work to a CPU index range."),
  opt("cpu-strict", ["--cpu-strict"], "Strict CPU placement", "Runtime", "required", "Controls whether CPU placement is strict for generation threads."),
  opt("prio", ["--prio"], "Thread priority", "Runtime", "required", "Sets generation thread priority."),
  opt("poll", ["--poll"], "Polling level", "Runtime", "required", "Controls polling while waiting for work."),
  opt("cpu-mask-batch", ["-Cb", "--cpu-mask-batch"], "Batch CPU mask", "Runtime", "required", "Pins batch work to CPUs selected by a hexadecimal affinity mask."),
  opt("cpu-range-batch", ["-Crb", "--cpu-range-batch"], "Batch CPU range", "Runtime", "required", "Pins batch work to a CPU index range."),
  opt("cpu-strict-batch", ["--cpu-strict-batch"], "Strict batch CPU placement", "Runtime", "required", "Controls whether CPU placement is strict for batch threads."),
  opt("prio-batch", ["--prio-batch"], "Batch priority", "Runtime", "required", "Sets batch thread priority."),
  opt("poll-batch", ["--poll-batch"], "Batch polling", "Runtime", "required", "Controls polling while batch threads wait for work."),
  opt("ctx-size", ["-c", "--ctx-size"], "Context window", "Runtime", "required", "Sets the prompt context size; 0 lets the model metadata choose it."),
  opt("n-predict", ["-n", "--predict", "--n-predict"], "Prediction limit", "Runtime", "required", "Sets the default generated-token limit; -1 means unlimited."),
  opt("batch-size", ["-b", "--batch-size"], "Logical maximum batch size", "Runtime", "required", "Sets the logical maximum batch size."),
  opt("ubatch-size", ["-ub", "--ubatch-size"], "Physical maximum batch size", "Runtime", "required", "Sets the physical maximum batch size, also known as the micro-batch size."),
  opt("keep", ["--keep"], "Prompt tokens kept", "Runtime", "required", "Keeps this many prompt tokens when context must be shifted."),
  opt("swa-full", ["--swa-full"], "Full SWA cache", "Memory", "none", "Uses a full-size sliding-window-attention cache."),
  opt("flash-attn", ["-fa", "--flash-attn"], "Flash attention", "Memory", "optional", "Controls Flash Attention use: on, off, or auto."),
  opt("perf", ["--perf"], "Performance timings", "Advanced", "none", "Enables internal libllama performance timing output."),
  opt("no-perf", ["--no-perf"], "Performance timings", "Advanced", "none", "Disables internal libllama performance timing output."),
  opt("escape", ["-e", "--escape"], "Escape processing", "Runtime", "none", "Processes escaped sequences such as newline and tab in prompts."),
  opt("no-escape", ["--no-escape"], "Escape processing", "Runtime", "none", "Leaves escaped prompt sequences unchanged."),
  opt("rope-scaling", ["--rope-scaling"], "RoPE scaling", "Runtime", "required", "Selects RoPE scaling mode, commonly none, linear, or yarn."),
  opt("rope-scale", ["--rope-scale"], "RoPE scale", "Runtime", "required", "Expands context by the given RoPE scale factor."),
  opt("rope-freq-base", ["--rope-freq-base"], "RoPE frequency base", "Runtime", "required", "Overrides the base RoPE frequency."),
  opt("rope-freq-scale", ["--rope-freq-scale"], "RoPE frequency scale", "Runtime", "required", "Scales RoPE frequency; larger context uses lower frequency scale."),
  opt("yarn-orig-ctx", ["--yarn-orig-ctx"], "YaRN original context", "Runtime", "required", "Sets the model training context used by YaRN scaling."),
  opt("yarn-ext-factor", ["--yarn-ext-factor"], "YaRN extension factor", "Runtime", "required", "Sets YaRN extrapolation and interpolation mix."),
  opt("yarn-attn-factor", ["--yarn-attn-factor"], "YaRN attention factor", "Runtime", "required", "Scales attention magnitude for YaRN."),
  opt("yarn-beta-slow", ["--yarn-beta-slow"], "YaRN beta slow", "Runtime", "required", "Sets YaRN high correction dimension."),
  opt("yarn-beta-fast", ["--yarn-beta-fast"], "YaRN beta fast", "Runtime", "required", "Sets YaRN low correction dimension."),
  opt("kv-offload", ["-kvo", "--kv-offload"], "KV cache offload", "Memory", "none", "Allows KV cache storage to use accelerator memory."),
  opt("no-kv-offload", ["-nkvo", "--no-kv-offload"], "KV cache offload", "Memory", "none", "Keeps KV cache storage off accelerator memory."),
  opt("repack", ["--repack"], "Weight repacking", "Memory", "none", "Enables weight repacking for faster kernels where supported."),
  opt("no-repack", ["-nr", "--no-repack"], "Weight repacking", "Memory", "none", "Disables model weight repacking."),
  opt("no-host", ["--no-host"], "Host buffer bypass", "Memory", "none", "Bypasses host buffers so extra backend buffers can be used."),
  opt("cache-type-k", ["-ctk", "--cache-type-k"], "K cache type", "Memory", "required", "Sets the KV cache data type for K tensors."),
  opt("cache-type-v", ["-ctv", "--cache-type-v"], "V cache type", "Memory", "required", "Sets the KV cache data type for V tensors."),
  opt("defrag-thold", ["-dt", "--defrag-thold"], "Defrag threshold", "Memory", "required", "Deprecated KV cache defragmentation threshold."),
  opt("mlock", ["--mlock"], "Lock model memory", "Memory", "none", "Keeps model memory resident instead of allowing swap or compression."),
  opt("mmap", ["--mmap"], "Memory map", "Memory", "none", "Loads model weights with memory mapping."),
  opt("no-mmap", ["--no-mmap"], "Memory map", "Memory", "none", "Disables memory mapping; loading may be slower but pageout behavior can improve."),
  opt("direct-io", ["-dio", "--direct-io"], "Direct IO", "Memory", "none", "Uses DirectIO where the platform supports it."),
  opt("no-direct-io", ["-ndio", "--no-direct-io"], "Direct IO", "Memory", "none", "Disables DirectIO."),
  opt("numa", ["--numa"], "NUMA strategy", "Runtime", "required", "Applies a NUMA placement strategy such as distribute, isolate, or numactl."),
  opt("device", ["-dev", "--device"], "Devices", "Memory", "required", "Selects accelerator devices for offloading."),
  opt("list-devices", ["--list-devices"], "List devices", "General", "none", "Prints available accelerator devices and exits."),
  opt("override-tensor", ["-ot", "--override-tensor"], "Tensor override", "Advanced", "required", "Overrides tensor buffer placement by name pattern."),
  opt("cpu-moe", ["-cmoe", "--cpu-moe"], "MoE on CPU", "Memory", "none", "Keeps all Mixture-of-Experts weights on CPU."),
  opt("n-cpu-moe", ["-ncmoe", "--n-cpu-moe"], "MoE CPU layers", "Memory", "required", "Keeps the first N MoE layers on CPU."),
  opt("n-gpu-layers", ["-ngl", "--gpu-layers", "--n-gpu-layers"], "GPU layers", "Memory", "required", "Sets how many model layers are offloaded to VRAM; auto and all are supported."),
  opt("split-mode", ["-sm", "--split-mode"], "GPU split mode", "Memory", "required", "Controls how model tensors are split across multiple GPUs."),
  opt("tensor-split", ["-ts", "--tensor-split"], "Tensor split", "Memory", "required", "Sets per-GPU offload proportions."),
  opt("main-gpu", ["-mg", "--main-gpu"], "Main GPU", "Memory", "required", "Selects the primary GPU for single-GPU split mode or intermediate work."),
  opt("fit", ["-fit", "--fit"], "Fit to memory", "Memory", "optional", "Adjusts unset arguments so the model fits available device memory."),
  opt("fit-target", ["-fitt", "--fit-target"], "Fit target margin", "Memory", "required", "Sets target free memory margin per device for fitting."),
  opt("fit-ctx", ["-fitc", "--fit-ctx"], "Fit minimum context", "Memory", "required", "Sets the minimum context size allowed while fitting to memory."),
  opt("check-tensors", ["--check-tensors"], "Check tensors", "Advanced", "none", "Checks loaded tensor data for invalid values."),
  opt("override-kv", ["--override-kv"], "Metadata override", "Advanced", "required", "Overrides model metadata key values."),
  opt("op-offload", ["--op-offload"], "Operation offload", "Memory", "none", "Offloads supported host tensor operations to device."),
  opt("no-op-offload", ["--no-op-offload"], "Operation offload", "Memory", "none", "Keeps host tensor operations on host."),
  opt("lora", ["--lora"], "LoRA adapters", "Model", "required", "Loads one or more LoRA adapter files."),
  opt("lora-scaled", ["--lora-scaled"], "Scaled LoRA adapters", "Model", "required", "Loads LoRA adapters with explicit scale values."),
  opt("control-vector", ["--control-vector"], "Control vector", "Model", "required", "Adds one or more control vector files."),
  opt("control-vector-scaled", ["--control-vector-scaled"], "Scaled control vector", "Model", "required", "Adds control vectors with explicit scale values."),
  opt("control-vector-layer-range", ["--control-vector-layer-range"], "Control vector layers", "Model", "required", "Limits control vector application to an inclusive layer range.", 2),
  opt("model", ["-m", "--model"], "Model file", "Model", "required", "Loads a local model file."),
  opt("model-url", ["-mu", "--model-url"], "Model URL", "Model", "required", "Downloads a model from a URL before loading it."),
  opt("docker-repo", ["-dr", "--docker-repo"], "Docker model repo", "Model", "required", "Loads a model from a Docker Hub model repository."),
  opt("hf-repo", ["-hf", "-hfr", "--hf-repo"], "Hugging Face repo", "Model", "required", "Downloads and loads a GGUF model from Hugging Face."),
  opt("hf-repo-draft", ["-hfd", "-hfrd", "--hf-repo-draft"], "Draft Hugging Face repo", "Speculative", "required", "Downloads the speculative decoding draft model from Hugging Face."),
  opt("hf-file", ["-hff", "--hf-file"], "Hugging Face file", "Model", "required", "Selects a specific GGUF file within the Hugging Face repo."),
  opt("hf-repo-v", ["-hfv", "-hfrv", "--hf-repo-v"], "Vocoder Hugging Face repo", "Model", "required", "Downloads a vocoder model from Hugging Face."),
  opt("hf-file-v", ["-hffv", "--hf-file-v"], "Vocoder Hugging Face file", "Model", "required", "Selects a specific vocoder file within the Hugging Face repo."),
  opt("hf-token", ["-hft", "--hf-token"], "Hugging Face token", "Model", "required", "Uses a Hugging Face access token for model download."),
  opt("log-disable", ["--log-disable"], "Disable logs", "Logging", "none", "Disables logging."),
  opt("log-file", ["--log-file"], "Log file", "Logging", "required", "Writes logs to a file."),
  opt("log-colors", ["--log-colors"], "Log colors", "Logging", "optional", "Controls colored log output."),
  opt("verbose", ["-v", "--verbose", "--log-verbose"], "Verbose logging", "Logging", "none", "Enables maximum verbosity."),
  opt("offline", ["--offline"], "Offline mode", "Model", "none", "Uses cache only and avoids network access."),
  opt("verbosity", ["-lv", "--verbosity", "--log-verbosity"], "Log verbosity", "Logging", "required", "Sets the log verbosity threshold."),
  opt("log-prefix", ["--log-prefix"], "Log prefix", "Logging", "none", "Adds prefixes to log messages."),
  opt("log-timestamps", ["--log-timestamps"], "Log timestamps", "Logging", "none", "Adds timestamps to log messages."),
  opt("cache-type-k-draft", ["-ctkd", "--cache-type-k-draft"], "Draft K cache type", "Speculative", "required", "Sets the draft model K cache data type."),
  opt("cache-type-v-draft", ["-ctvd", "--cache-type-v-draft"], "Draft V cache type", "Speculative", "required", "Sets the draft model V cache data type."),

  opt("samplers", ["--samplers"], "Sampler chain", "Sampling", "required", "Sets generation samplers and their order."),
  opt("seed", ["-s", "--seed"], "Seed", "Sampling", "required", "Sets the random seed; -1 chooses a random seed."),
  opt("sampler-seq", ["--sampler-seq", "--sampling-seq"], "Sampler sequence", "Sampling", "required", "Sets a compact sampler order sequence."),
  opt("ignore-eos", ["--ignore-eos"], "Ignore EOS", "Sampling", "none", "Continues generation after the end-of-stream token."),
  opt("temperature", ["--temp", "--temperature"], "Temperature", "Sampling", "required", "Controls generation randomness; lower is more deterministic."),
  opt("top-k", ["--top-k"], "Top K", "Sampling", "required", "Samples only from the K most likely tokens."),
  opt("top-p", ["--top-p"], "Top P", "Sampling", "required", "Samples from tokens within a cumulative probability mass."),
  opt("min-p", ["--min-p"], "Min P", "Sampling", "required", "Filters low-probability tokens relative to the most likely token."),
  opt("top-n-sigma", ["--top-nsigma", "--top-n-sigma"], "Top N sigma", "Sampling", "required", "Filters tokens using a sigma-based probability threshold."),
  opt("xtc-probability", ["--xtc-probability"], "XTC probability", "Sampling", "required", "Sets XTC sampling probability."),
  opt("xtc-threshold", ["--xtc-threshold"], "XTC threshold", "Sampling", "required", "Sets XTC sampling threshold."),
  opt("typical-p", ["--typical", "--typical-p"], "Typical P", "Sampling", "required", "Enables locally typical sampling with the given probability."),
  opt("repeat-last-n", ["--repeat-last-n"], "Repeat window", "Sampling", "required", "Sets how many recent tokens are considered for repetition penalty."),
  opt("repeat-penalty", ["--repeat-penalty"], "Repeat penalty", "Sampling", "required", "Penalizes repeated token sequences."),
  opt("presence-penalty", ["--presence-penalty"], "Presence penalty", "Sampling", "required", "Penalizes tokens that have already appeared."),
  opt("frequency-penalty", ["--frequency-penalty"], "Frequency penalty", "Sampling", "required", "Penalizes tokens based on repetition frequency."),
  opt("dry-multiplier", ["--dry-multiplier"], "DRY multiplier", "Sampling", "required", "Sets strength for DRY repetition reduction."),
  opt("dry-base", ["--dry-base"], "DRY base", "Sampling", "required", "Sets the base value for DRY repetition reduction."),
  opt("dry-allowed-length", ["--dry-allowed-length"], "DRY allowed length", "Sampling", "required", "Allows short repeated spans before DRY penalty applies."),
  opt("dry-penalty-last-n", ["--dry-penalty-last-n"], "DRY penalty window", "Sampling", "required", "Sets the recent-token window for DRY penalty."),
  opt("dry-sequence-breaker", ["--dry-sequence-breaker"], "DRY sequence breaker", "Sampling", "required", "Adds sequence breakers that reset DRY repetition tracking."),
  opt("adaptive-target", ["--adaptive-target"], "Adaptive P target", "Sampling", "required", "Selects tokens near an adaptive probability target."),
  opt("adaptive-decay", ["--adaptive-decay"], "Adaptive P decay", "Sampling", "required", "Controls how quickly adaptive sampling target changes."),
  opt("dynatemp-range", ["--dynatemp-range"], "Dynamic temperature range", "Sampling", "required", "Allows temperature to vary within a configured range."),
  opt("dynatemp-exp", ["--dynatemp-exp"], "Dynamic temperature exponent", "Sampling", "required", "Shapes the dynamic temperature curve."),
  opt("mirostat", ["--mirostat"], "Mirostat", "Sampling", "required", "Enables Mirostat sampling mode."),
  opt("mirostat-lr", ["--mirostat-lr"], "Mirostat learning rate", "Sampling", "required", "Sets Mirostat learning rate."),
  opt("mirostat-ent", ["--mirostat-ent"], "Mirostat target entropy", "Sampling", "required", "Sets Mirostat target entropy."),
  opt("logit-bias", ["-l", "--logit-bias"], "Logit bias", "Sampling", "required", "Adjusts the likelihood of a specific token ID."),
  opt("grammar", ["--grammar"], "Grammar", "Sampling", "required", "Constrains generation with an inline grammar."),
  opt("grammar-file", ["--grammar-file"], "Grammar file", "Sampling", "required", "Constrains generation with a grammar loaded from a file."),
  opt("json-schema", ["-j", "--json-schema"], "JSON schema", "Sampling", "required", "Constrains generation to a JSON schema."),
  opt("json-schema-file", ["-jf", "--json-schema-file"], "JSON schema file", "Sampling", "required", "Constrains generation using a JSON schema file."),
  opt("backend-sampling", ["-bs", "--backend-sampling"], "Backend sampling", "Sampling", "none", "Enables experimental backend sampling."),

  opt("lookup-cache-static", ["-lcs", "--lookup-cache-static"], "Static lookup cache", "Server", "required", "Reads lookup decoding cache from a static file."),
  opt("lookup-cache-dynamic", ["-lcd", "--lookup-cache-dynamic"], "Dynamic lookup cache", "Server", "required", "Reads and updates lookup decoding cache from a dynamic file."),
  opt("ctx-checkpoints", ["-ctxcp", "--ctx-checkpoints", "--swa-checkpoints"], "Context checkpoints", "Server", "required", "Sets maximum context checkpoints per slot."),
  opt("checkpoint-every-n-tokens", ["-cpent", "--checkpoint-every-n-tokens"], "Checkpoint interval", "Server", "required", "Creates a checkpoint every N prefill tokens."),
  opt("cache-ram", ["-cram", "--cache-ram"], "RAM cache limit", "Server", "required", "Limits server RAM cache in MiB."),
  opt("kv-unified", ["-kvu", "--kv-unified"], "Unified KV cache", "Server", "none", "Uses one KV buffer shared across sequences."),
  opt("no-kv-unified", ["-no-kvu", "--no-kv-unified"], "Unified KV cache", "Server", "none", "Disables a single shared KV buffer."),
  opt("cache-idle-slots", ["--cache-idle-slots"], "Cache idle slots", "Server", "none", "Saves and clears idle slots on new tasks."),
  opt("no-cache-idle-slots", ["--no-cache-idle-slots"], "Cache idle slots", "Server", "none", "Disables idle-slot caching."),
  opt("context-shift", ["--context-shift"], "Context shift", "Server", "none", "Allows context shifting during long generation."),
  opt("no-context-shift", ["--no-context-shift"], "Context shift", "Server", "none", "Disables context shifting."),
  opt("reverse-prompt", ["-r", "--reverse-prompt"], "Reverse prompt", "Server", "required", "Stops generation at a prompt marker."),
  opt("special", ["-sp", "--special"], "Special tokens", "Server", "none", "Allows special-token output."),
  opt("warmup", ["--warmup"], "Warmup", "Server", "none", "Runs a warmup pass before serving."),
  opt("no-warmup", ["--no-warmup"], "Warmup", "Server", "none", "Skips the warmup pass."),
  opt("spm-infill", ["--spm-infill"], "SPM infill", "Server", "none", "Uses Suffix/Prefix/Middle ordering for infill."),
  opt("pooling", ["--pooling"], "Pooling", "Embeddings", "required", "Sets embedding pooling strategy."),
  opt("parallel", ["-np", "--parallel"], "Parallel slots", "Server", "required", "Sets the number of server slots for concurrent work."),
  opt("cont-batching", ["-cb", "--cont-batching"], "Continuous batching", "Server", "none", "Enables dynamic batching across requests."),
  opt("no-cont-batching", ["-nocb", "--no-cont-batching"], "Continuous batching", "Server", "none", "Disables dynamic batching."),
  opt("mmproj", ["-mm", "--mmproj"], "Multimodal projector", "Model", "required", "Loads a multimodal projector file."),
  opt("mmproj-url", ["-mmu", "--mmproj-url"], "Multimodal projector URL", "Model", "required", "Downloads a multimodal projector file."),
  opt("mmproj-auto", ["--mmproj-auto"], "Auto multimodal projector", "Model", "none", "Automatically uses an available multimodal projector."),
  opt("no-mmproj", ["--no-mmproj", "--no-mmproj-auto"], "Auto multimodal projector", "Model", "none", "Disables automatic multimodal projector use."),
  opt("mmproj-offload", ["--mmproj-offload"], "Projector offload", "Memory", "none", "Offloads multimodal projector work to GPU where supported."),
  opt("no-mmproj-offload", ["--no-mmproj-offload"], "Projector offload", "Memory", "none", "Keeps multimodal projector work off GPU."),
  opt("image-min-tokens", ["--image-min-tokens"], "Image min tokens", "Model", "required", "Sets minimum tokens per dynamic-resolution image."),
  opt("image-max-tokens", ["--image-max-tokens"], "Image max tokens", "Model", "required", "Sets maximum tokens per dynamic-resolution image."),
  opt("override-tensor-draft", ["-otd", "--override-tensor-draft"], "Draft tensor override", "Speculative", "required", "Overrides draft-model tensor buffer placement."),
  opt("cpu-moe-draft", ["-cmoed", "--cpu-moe-draft"], "Draft MoE on CPU", "Speculative", "none", "Keeps all draft Mixture-of-Experts weights on CPU."),
  opt("n-cpu-moe-draft", ["-ncmoed", "--n-cpu-moe-draft"], "Draft MoE CPU layers", "Speculative", "required", "Keeps the first N draft MoE layers on CPU."),
  opt("alias", ["-a", "--alias"], "Server model alias", "Server", "required", "Sets model aliases visible to the API."),
  opt("tags", ["--tags"], "Model tags", "Server", "required", "Adds informational model tags."),
  opt("host", ["--host"], "Host", "Server", "required", "Sets the network address or Unix socket to bind."),
  opt("port", ["--port"], "Port", "Server", "required", "Sets the HTTP listen port."),
  opt("reuse-port", ["--reuse-port"], "Reuse port", "Server", "none", "Allows multiple sockets to bind to the same port."),
  opt("path", ["--path"], "Static path", "Server", "required", "Serves web assets from a custom static path."),
  opt("api-prefix", ["--api-prefix"], "API prefix", "Server", "required", "Serves API routes below a URL prefix."),
  opt("webui-config", ["--webui-config"], "Web UI config", "Server", "required", "Applies default Web UI preferences from JSON."),
  opt("webui-config-file", ["--webui-config-file"], "Web UI config file", "Server", "required", "Applies default Web UI preferences from a JSON file."),
  opt("webui-mcp-proxy", ["--webui-mcp-proxy"], "Web UI MCP proxy", "Server", "none", "Enables the experimental MCP CORS proxy."),
  opt("no-webui-mcp-proxy", ["--no-webui-mcp-proxy"], "Web UI MCP proxy", "Server", "none", "Disables the MCP CORS proxy."),
  opt("tools", ["--tools"], "Built-in tools", "Server", "required", "Enables selected built-in Web UI tools."),
  opt("webui", ["--webui"], "Web UI", "Server", "none", "Enables the llama-server Web UI."),
  opt("no-webui", ["--no-webui"], "Web UI", "Server", "none", "Disables the llama-server Web UI."),
  opt("embedding", ["--embedding", "--embeddings"], "Embeddings only", "Embeddings", "none", "Restricts the server to embedding use cases."),
  opt("reranking", ["--rerank", "--reranking"], "Reranking", "Embeddings", "none", "Enables the reranking endpoint."),
  opt("api-key", ["--api-key"], "API key", "Server", "required", "Sets one or more API keys for llama-server authentication."),
  opt("api-key-file", ["--api-key-file"], "API key file", "Server", "required", "Loads API keys from a file."),
  opt("ssl-key-file", ["--ssl-key-file"], "SSL key file", "Server", "required", "Loads a PEM SSL private key."),
  opt("ssl-cert-file", ["--ssl-cert-file"], "SSL certificate file", "Server", "required", "Loads a PEM SSL certificate."),
  opt("chat-template-kwargs", ["--chat-template-kwargs"], "Chat template kwargs", "Chat", "required", "Passes JSON parameters into the chat template parser."),
  opt("timeout", ["-to", "--timeout"], "HTTP timeout", "Server", "required", "Sets server read and write timeout in seconds."),
  opt("threads-http", ["--threads-http"], "HTTP threads", "Server", "required", "Sets worker threads for HTTP request processing."),
  opt("cache-prompt", ["--cache-prompt"], "Prompt cache", "Server", "none", "Enables prompt caching."),
  opt("no-cache-prompt", ["--no-cache-prompt"], "Prompt cache", "Server", "none", "Disables prompt caching."),
  opt("cache-reuse", ["--cache-reuse"], "Cache reuse chunk", "Server", "required", "Sets minimum chunk size for cache reuse."),
  opt("metrics", ["--metrics"], "Metrics endpoint", "Server", "none", "Enables Prometheus-compatible metrics."),
  opt("props", ["--props"], "Props endpoint", "Server", "none", "Enables changing global properties through /props."),
  opt("slots", ["--slots"], "Slots endpoint", "Server", "none", "Exposes slot monitoring."),
  opt("no-slots", ["--no-slots"], "Slots endpoint", "Server", "none", "Disables slot monitoring."),
  opt("slot-save-path", ["--slot-save-path"], "Slot save path", "Server", "required", "Sets where slot KV cache snapshots are saved."),
  opt("media-path", ["--media-path"], "Media path", "Server", "required", "Sets local media directory for file URLs."),
  opt("models-dir", ["--models-dir"], "Models directory", "Server", "required", "Sets model directory for llama-server router mode."),
  opt("models-preset", ["--models-preset"], "Models preset", "Server", "required", "Loads router model presets from an INI file."),
  opt("models-max", ["--models-max"], "Maximum loaded models", "Server", "required", "Limits loaded models in router mode."),
  opt("models-autoload", ["--models-autoload"], "Model autoload", "Server", "none", "Automatically loads requested router models."),
  opt("no-models-autoload", ["--no-models-autoload"], "Model autoload", "Server", "none", "Disables automatic router model loading."),
  opt("jinja", ["--jinja"], "Jinja templates", "Chat", "none", "Enables Jinja chat templates."),
  opt("no-jinja", ["--no-jinja"], "Jinja templates", "Chat", "none", "Disables Jinja chat templates."),
  opt("reasoning-format", ["--reasoning-format"], "Reasoning format", "Chat", "required", "Controls how thought content is parsed and returned."),
  opt("reasoning", ["-rea", "--reasoning"], "Reasoning", "Chat", "optional", "Controls reasoning mode: on, off, or auto."),
  opt("reasoning-budget", ["--reasoning-budget"], "Reasoning token budget", "Chat", "required", "Limits thinking tokens; -1 means unrestricted."),
  opt("reasoning-budget-message", ["--reasoning-budget-message"], "Reasoning budget message", "Chat", "required", "Injects a message when the reasoning budget is exhausted."),
  opt("chat-template", ["--chat-template"], "Chat template", "Chat", "required", "Selects a built-in or custom chat template."),
  opt("chat-template-file", ["--chat-template-file"], "Chat template file", "Chat", "required", "Loads a chat template from a file."),
  opt("skip-chat-parsing", ["--skip-chat-parsing"], "Skip chat parsing", "Chat", "none", "Uses a pure content parser even with a Jinja template."),
  opt("no-skip-chat-parsing", ["--no-skip-chat-parsing"], "Skip chat parsing", "Chat", "none", "Allows normal chat parsing."),
  opt("prefill-assistant", ["--prefill-assistant"], "Assistant prefill", "Chat", "none", "Prefills an assistant response when the final message is assistant role."),
  opt("no-prefill-assistant", ["--no-prefill-assistant"], "Assistant prefill", "Chat", "none", "Disables assistant response prefill."),
  opt("slot-prompt-similarity", ["-sps", "--slot-prompt-similarity"], "Slot prompt similarity", "Server", "required", "Sets required prompt similarity for slot reuse."),
  opt("lora-init-without-apply", ["--lora-init-without-apply"], "Load LoRA inactive", "Model", "none", "Loads LoRA adapters but leaves applying them to the API."),
  opt("sleep-idle-seconds", ["--sleep-idle-seconds"], "Sleep on idle", "Server", "required", "Sleeps after the configured idle time; -1 disables it."),
  opt("threads-draft", ["-td", "--threads-draft"], "Draft threads", "Speculative", "required", "Sets generation threads for the draft model."),
  opt("threads-batch-draft", ["-tbd", "--threads-batch-draft"], "Draft batch threads", "Speculative", "required", "Sets batch threads for the draft model."),
  opt("draft-max", ["--draft", "--draft-n", "--draft-max"], "Draft tokens", "Speculative", "required", "Sets maximum tokens drafted for speculative decoding."),
  opt("draft-min", ["--draft-min", "--draft-n-min"], "Minimum draft tokens", "Speculative", "required", "Sets minimum speculative draft tokens."),
  opt("draft-p-min", ["--draft-p-min"], "Minimum draft probability", "Speculative", "required", "Sets minimum greedy speculative decoding probability."),
  opt("ctx-size-draft", ["-cd", "--ctx-size-draft"], "Draft context window", "Speculative", "required", "Sets draft-model context size."),
  opt("device-draft", ["-devd", "--device-draft"], "Draft devices", "Speculative", "required", "Selects devices for draft model offloading."),
  opt("n-gpu-layers-draft", ["-ngld", "--gpu-layers-draft", "--n-gpu-layers-draft"], "Draft GPU layers", "Speculative", "required", "Sets draft-model layers offloaded to VRAM."),
  opt("model-draft", ["-md", "--model-draft"], "Draft model", "Speculative", "required", "Loads a draft model for speculative decoding."),
  opt("spec-replace", ["--spec-replace"], "Speculative string replacement", "Speculative", "required", "Maps target model strings to draft model strings.", 2),
  opt("spec-type", ["--spec-type"], "Speculative type", "Speculative", "required", "Selects speculative decoding strategy when no draft model is supplied."),
  opt("spec-ngram-size-n", ["--spec-ngram-size-n"], "Speculative ngram N", "Speculative", "required", "Sets lookup n-gram length for n-gram speculation."),
  opt("spec-ngram-size-m", ["--spec-ngram-size-m"], "Speculative ngram M", "Speculative", "required", "Sets draft m-gram length for n-gram speculation."),
  opt("spec-ngram-min-hits", ["--spec-ngram-min-hits"], "Speculative minimum hits", "Speculative", "required", "Sets minimum hits for n-gram map speculation."),
  opt("model-vocoder", ["-mv", "--model-vocoder"], "Vocoder model", "Model", "required", "Loads a vocoder model for audio generation."),
  opt("tts-use-guide-tokens", ["--tts-use-guide-tokens"], "TTS guide tokens", "Model", "none", "Uses guide tokens to improve TTS word recall."),
  opt("embd-gemma-default", ["--embd-gemma-default"], "EmbeddingGemma default", "Model", "none", "Uses the built-in EmbeddingGemma model preset."),
  opt("fim-qwen-1.5b-default", ["--fim-qwen-1.5b-default"], "Qwen FIM 1.5B preset", "Model", "none", "Uses the built-in Qwen 2.5 Coder 1.5B FIM preset."),
  opt("fim-qwen-3b-default", ["--fim-qwen-3b-default"], "Qwen FIM 3B preset", "Model", "none", "Uses the built-in Qwen 2.5 Coder 3B FIM preset."),
  opt("fim-qwen-7b-default", ["--fim-qwen-7b-default"], "Qwen FIM 7B preset", "Model", "none", "Uses the built-in Qwen 2.5 Coder 7B FIM preset."),
  opt("fim-qwen-7b-spec", ["--fim-qwen-7b-spec"], "Qwen FIM 7B speculative preset", "Model", "none", "Uses the built-in Qwen 2.5 Coder 7B speculative preset."),
  opt("fim-qwen-14b-spec", ["--fim-qwen-14b-spec"], "Qwen FIM 14B speculative preset", "Model", "none", "Uses the built-in Qwen 2.5 Coder 14B speculative preset."),
  opt("fim-qwen-30b-default", ["--fim-qwen-30b-default"], "Qwen FIM 30B preset", "Model", "none", "Uses the built-in Qwen 3 Coder 30B A3B preset."),
  opt("gpt-oss-20b-default", ["--gpt-oss-20b-default"], "gpt-oss 20B preset", "Model", "none", "Uses the built-in gpt-oss-20b preset."),
  opt("gpt-oss-120b-default", ["--gpt-oss-120b-default"], "gpt-oss 120B preset", "Model", "none", "Uses the built-in gpt-oss-120b preset."),
  opt("vision-gemma-4b-default", ["--vision-gemma-4b-default"], "Gemma vision 4B preset", "Model", "none", "Uses the built-in Gemma 3 4B vision preset."),
  opt("vision-gemma-12b-default", ["--vision-gemma-12b-default"], "Gemma vision 12B preset", "Model", "none", "Uses the built-in Gemma 3 12B vision preset."),
]);

const optionByAlias = new Map<string, LlamaServerOption>();
for (const option of LLAMA_SERVER_OPTIONS) {
  for (const alias of option.aliases) optionByAlias.set(alias, option);
}

const categoryOrder: OptionCategory[] = [
  "Model",
  "Runtime",
  "Memory",
  "Server",
  "Sampling",
  "Embeddings",
  "Chat",
  "Speculative",
  "Logging",
  "Advanced",
  "General",
];

function opt(
  id: string,
  aliases: string[],
  label: string,
  category: OptionCategory,
  valueMode: OptionValueMode,
  explanation: string,
  valueCount = 1,
): LlamaServerOption {
  return { id, aliases, label, category, valueMode, valueCount, explanation };
}

export function interpretLlamaServerCommand(command: string): ModelConfigInterpretation {
  const args = parseCommandLine(command);
  const executable = args[0] ?? "";
  const parsedOptions: ParsedOption[] = [];
  const unknownOptions: UnknownOption[] = [];
  const positionals: string[] = [];

  for (let index = 1; index < args.length; index++) {
    const token = args[index];
    if (!isFlag(token)) {
      positionals.push(token);
      continue;
    }

    const { flag, inlineValue } = splitInlineFlag(token);
    const option = optionByAlias.get(flag);
    if (!option) {
      const value = inlineValue ?? readUnknownValue(args, index);
      if (value !== null && inlineValue === null) index += 1;
      unknownOptions.push({ flag, value, raw: value === null ? token : `${flag} ${value}` });
      continue;
    }

    const values: string[] = [];
    if (inlineValue !== null) {
      values.push(inlineValue);
    } else if (option.valueMode === "required") {
      const count = option.valueCount ?? 1;
      for (let offset = 0; offset < count && index + 1 < args.length; offset++) {
        const next = args[index + 1];
        if (isFlag(next) && values.length === 0) break;
        values.push(next);
        index += 1;
      }
    } else if (option.valueMode === "optional" && index + 1 < args.length && !isFlag(args[index + 1])) {
      values.push(args[index + 1]);
      index += 1;
    }

    const value = values.length > 0 ? values.join(" ") : null;
    parsedOptions.push({
      option,
      flag,
      value,
      raw: value === null ? flag : `${flag} ${value}`,
      enabled: option.valueMode === "none" ? !isNegativeFlag(flag) : parseBooleanValue(option.id, flag, value),
    });
  }

  return {
    provider: detectProvider(executable, args, parsedOptions),
    executable,
    args,
    options: parsedOptions,
    unknownOptions,
    positionals,
    highlights: buildHighlights(parsedOptions),
    categories: groupOptions(parsedOptions),
  };
}

export function parseCommandLine(command: string): string[] {
  const tokens: string[] = [];
  let current = "";
  let quote: '"' | "'" | null = null;
  let escaping = false;

  for (const char of command) {
    if (escaping) {
      current += char;
      escaping = false;
      continue;
    }

    if (char === "\\" && quote !== "'") {
      escaping = true;
      continue;
    }

    if ((char === '"' || char === "'") && quote === null) {
      quote = char;
      continue;
    }

    if (char === quote) {
      quote = null;
      continue;
    }

    if (/\s/.test(char) && quote === null) {
      if (current !== "") {
        tokens.push(current);
        current = "";
      }
      continue;
    }

    current += char;
  }

  if (escaping) current += "\\";
  if (current !== "") tokens.push(current);
  return tokens;
}

export function getOptionSourceLabel(): string {
  return source;
}

function splitInlineFlag(token: string): { flag: string; inlineValue: string | null } {
  const eqIndex = token.indexOf("=");
  if (eqIndex <= 0) return { flag: token, inlineValue: null };
  return { flag: token.slice(0, eqIndex), inlineValue: token.slice(eqIndex + 1) };
}

function isFlag(token: string): boolean {
  return token.startsWith("-") && token !== "-";
}

function readUnknownValue(args: string[], index: number): string | null {
  const next = args[index + 1];
  if (!next || isFlag(next)) return null;
  return next;
}

function isNegativeFlag(flag: string): boolean {
  return flag.startsWith("--no-") || flag.startsWith("-no-") || /^-n[a-z]/.test(flag);
}

function parseBooleanValue(optionID: string, flag: string, value: string | null): boolean | null {
  if (value === null) return null;
  const normalized = value.toLowerCase();
  if (["1", "true", "on", "yes", "enabled"].includes(normalized)) return !isNegativeFlag(flag);
  if (["0", "false", "off", "no", "disabled"].includes(normalized)) return isNegativeFlag(flag);
  if (optionID === "flash-attn" && normalized === "auto") return null;
  return null;
}

function detectProvider(
  executable: string,
  args: string[],
  parsedOptions: ParsedOption[],
): ModelConfigInterpretation["provider"] {
  const executableName = executable.split(/[\\/]/).pop()?.toLowerCase() ?? "";
  if (executableName.includes("llama-server") || executableName === "server") return "llama.cpp";
  if (args.some((arg) => arg.includes("llama.cpp:server") || arg.includes("ghcr.io/ggml-org/llama.cpp"))) return "llama.cpp";
  if (parsedOptions.some((item) => ["ctx-size", "flash-attn", "n-gpu-layers", "hf-repo"].includes(item.option.id))) {
    return "llama.cpp";
  }
  return "unknown";
}

function buildHighlights(options: ParsedOption[]): ConfigHighlight[] {
  const byID = new Map<string, ParsedOption>();
  for (const option of options) byID.set(option.option.id, option);

  const highlights: ConfigHighlight[] = [];
  addHighlight(highlights, byID.get("model") ?? byID.get("hf-repo") ?? byID.get("model-url"), "Model source", "Model file, URL, or remote repository used by llama.cpp.", "neutral");
  addVisionHighlight(highlights, byID);
  addHighlight(highlights, byID.get("ctx-size"), "Context", "Prompt context window configured for the server.", "neutral", (value) => `${formatNumber(value)} tokens`);
  addHighlight(highlights, byID.get("flash-attn"), "Flash attention", "Attention kernels are controlled explicitly.", byID.get("flash-attn")?.enabled === false ? "warning" : "good", formatOnOffAuto);
  addHighlight(highlights, byID.get("n-gpu-layers"), "GPU layers", "Layer offload setting for VRAM acceleration.", "good");
  addHighlight(highlights, byID.get("parallel"), "Parallel slots", "Concurrent server slots available for requests.", "neutral");
  addHighlight(highlights, byID.get("batch-size"), "Batch size", "Logical maximum batch size used during prompt processing.", "neutral");
  addHighlight(highlights, byID.get("ubatch-size"), "Micro-batch", "Physical maximum batch size, also known as the micro-batch size.", "neutral");
  addHighlight(highlights, byID.get("cache-type-k"), "K cache", "Data type for K tensors in KV cache.", "neutral");
  addHighlight(highlights, byID.get("cache-type-v"), "V cache", "Data type for V tensors in KV cache.", "neutral");
  addHighlight(highlights, byID.get("model-draft") ?? byID.get("hf-repo-draft"), "Speculative draft", "Draft model configured for speculative decoding.", "good");
  addHighlight(highlights, byID.get("embedding"), "Embeddings", "Server is restricted to embedding use.", "neutral", () => "enabled");
  addHighlight(highlights, byID.get("reranking"), "Reranking", "Reranking endpoint is enabled.", "neutral", () => "enabled");

  return highlights;
}

function addVisionHighlight(highlights: ConfigHighlight[], byID: Map<string, ParsedOption>): void {
  const projector = byID.get("mmproj") ?? byID.get("mmproj-url");
  const autoProjector = byID.get("mmproj-auto");
  const disabledProjector = byID.get("no-mmproj");

  if (disabledProjector) {
    highlights.push({
      id: "vision",
      label: "Vision",
      value: "disabled",
      note: "Multimodal projector loading is explicitly disabled.",
      tone: "warning",
    });
    return;
  }

  if (projector?.value) {
    highlights.push({
      id: "vision",
      label: "Vision",
      value: "enabled",
      note: `Multimodal processing is enabled with ${projector.value}.`,
      tone: "good",
    });
    return;
  }

  if (autoProjector) {
    highlights.push({
      id: "vision",
      label: "Vision",
      value: "auto",
      note: "llama.cpp will automatically use an available multimodal projector.",
      tone: "neutral",
    });
    return;
  }

  highlights.push({
    id: "vision",
    label: "Vision",
    value: "disabled",
    note: "No multimodal projector is configured.",
    tone: "neutral",
  });
}

function addHighlight(
  highlights: ConfigHighlight[],
  parsed: ParsedOption | undefined,
  label: string,
  note: string,
  tone: ConfigHighlight["tone"],
  format: (value: string | null, parsed: ParsedOption) => string = (value) => value ?? "enabled",
) {
  if (!parsed) return;
  highlights.push({
    id: parsed.option.id,
    label,
    value: format(parsed.value, parsed),
    note,
    tone,
  });
}

function formatOnOffAuto(value: string | null, parsed: ParsedOption): string {
  if (value === null) return parsed.enabled === false ? "off" : "on";
  const normalized = value.toLowerCase();
  if (["1", "true", "on", "yes", "enabled"].includes(normalized)) return "on";
  if (["0", "false", "off", "no", "disabled"].includes(normalized)) return "off";
  return value;
}

function formatNumber(value: string | null): string {
  if (value === null) return "enabled";
  const number = Number(value);
  if (!Number.isFinite(number)) return value;
  return new Intl.NumberFormat().format(number);
}

function groupOptions(options: ParsedOption[]): ModelConfigInterpretation["categories"] {
  const grouped = new Map<OptionCategory, ParsedOption[]>();
  for (const option of options) {
    const items = grouped.get(option.option.category) ?? [];
    items.push(option);
    grouped.set(option.option.category, items);
  }
  return categoryOrder
    .filter((category) => grouped.has(category))
    .map((category) => ({ category, options: grouped.get(category) ?? [] }));
}
