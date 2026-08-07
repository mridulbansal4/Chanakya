import { streamText } from 'ai';
import { createGoogleGenerativeAI } from '@ai-sdk/google';
import { NextRequest } from 'next/server';

export const dynamic = 'force-dynamic';

export async function POST(req: NextRequest) {
  try {
    const { prompt, system } = await req.json();

    const apiKey = process.env.GEMINI_API_KEY;
    if (!apiKey) {
      return new Response('GEMINI_API_KEY is not set', { status: 500 });
    }

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
