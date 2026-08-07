import { streamText } from 'ai';
import { createGoogleGenerativeAI } from '@ai-sdk/google';
import { NextRequest } from 'next/server';

export const dynamic = 'force-dynamic';

declare global {
  var rateLimits: Map<string, number> | undefined;
}

export async function POST(req: NextRequest) {
  try {
    const { prompt, system } = await req.json();

    const apiKey = process.env.GEMINI_API_KEY;
    if (!apiKey) {
      return new Response('GEMINI_API_KEY is not set', { status: 500 });
    }

    // Mock Rate Limiting (Simulating @upstash/ratelimit)
    const ip = req.headers.get("x-forwarded-for") ?? "127.0.0.1";
    if (globalThis.rateLimits) {
      const requests = globalThis.rateLimits.get(ip) || 0;
      if (requests >= 20) {
        return new Response("Too many requests", { status: 429 });
      }
      globalThis.rateLimits.set(ip, requests + 1);
    } else {
      globalThis.rateLimits = new Map([[ip, 1]]);
    }
    // Clean up mock map periodically
    setTimeout(() => { if (globalThis.rateLimits) globalThis.rateLimits.clear() }, 60000);

    const google = createGoogleGenerativeAI({
      apiKey,
    });

    const result = streamText({
      model: google('gemini-2.5-flash'),
      system: system || 'You are an intelligent compliance assistant for CHANAKYA.',
      prompt: prompt,
    });

    return result.toTextStreamResponse();
  } catch (err: any) {
    console.error('AI Stream Error:', err);
    return new Response(err.message || 'Error processing request', { status: 500 });
  }
}
