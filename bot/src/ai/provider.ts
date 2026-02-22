import type { LLMRequest, LLMResponse } from "../types.js";

export interface LLMProvider {
  complete(request: LLMRequest): Promise<LLMResponse>;
}
