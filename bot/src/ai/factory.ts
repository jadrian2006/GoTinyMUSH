import type { LLMProvider } from "./provider.js";
import { AnthropicProvider } from "./anthropic.js";
import { OpenAIProvider } from "./openai.js";

interface LLMConfig {
  provider: "anthropic" | "openai";
  apiKey: string;
  model: string;
  baseUrl?: string;
}

export function createProvider(config: LLMConfig): LLMProvider {
  switch (config.provider) {
    case "anthropic":
      return new AnthropicProvider(config.apiKey, config.model);
    case "openai":
      return new OpenAIProvider(config.apiKey, config.model, config.baseUrl);
    default:
      throw new Error(`Unknown LLM provider: ${config.provider}`);
  }
}
