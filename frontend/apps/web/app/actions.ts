"use server"

export async function explainClause(text: string): Promise<string> {
  const apiKey = process.env.GEMINI_API_KEY
  if (!apiKey) {
    throw new Error("GEMINI_API_KEY is not set in the environment.")
  }

  const prompt = `You are a helpful regulatory assistant. Summarize and explain this raw regulatory clause in very simple, plain English. Keep it strictly to 1 or 2 concise sentences so it's easy to understand at a glance:\n\n${text}`

  const res = await fetch(`https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=${apiKey}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      contents: [{
        parts: [{ text: prompt }]
      }]
    })
  })

  if (!res.ok) {
    const errorText = await res.text()
    console.error("Gemini API Error:", errorText)
    throw new Error("Failed to generate explanation from Gemini API.")
  }

  const data = await res.json()
  return data.candidates?.[0]?.content?.parts?.[0]?.text || "No explanation generated."
}
