import OpenAI from "openai";
import type { LLMProvider } from "./provider.js";
import type { LLMRequest, LLMResponse } from "../types.js";

export class OpenAIProvider implements LLMProvider {
  private client: OpenAI;

  constructor(
    apiKey: string,
    private readonly model: string,
    baseUrl?: string
  ) {
    this.client = new OpenAI({
      apiKey,
      ...(baseUrl ? { baseURL: baseUrl } : {}),
    });
  }

  async complete(request: LLMRequest): Promise<LLMResponse> {
    const response = await this.client.chat.completions.create({
      model: this.model,
      max_tokens: request.maxTokens,
      messages: [
        { role: "system", content: request.systemPrompt },
        ...request.messages.map((m) => ({
          role: m.role as "user" | "assistant",
          content: m.content,
        })),
      ],
    });

    const content = response.choices[0]?.message?.content || "";
    return {
      content,
      usage: response.usage
        ? {
            inputTokens: response.usage.prompt_tokens,
            outputTokens: response.usage.completion_tokens ?? 0,
          }
        : undefined,
    };
  }
}
