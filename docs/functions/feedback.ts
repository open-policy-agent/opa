import { Context } from "netlify:edge";

const headers = { "Content-Type": "application/json" };

export default async (req: Request, context: Context) => {
  if (req.method !== "POST") {
    return new Response(JSON.stringify({ error: "Method not allowed" }), {
      status: 405,
      headers,
    });
  }

  try {
    const requestBody = await req.json();
    const feedbackData = requestBody.data;
    const formName = requestBody.form_name;

    // Verify this is from the correct form
    if (formName !== "page-feedback") {
      return new Response(JSON.stringify({ error: "Invalid form" }), {
        status: 400,
        headers,
      });
    }

    const slackUrl = Deno.env.get("SLACK_DOCUMENTATION_FEEDBACK_URL");
    if (!slackUrl) {
      console.error("SLACK_DOCUMENTATION_FEEDBACK_URL environment variable not set");
      return new Response(JSON.stringify({ error: "Configuration error" }), {
        status: 500,
        headers,
      });
    }

    const feedbackType = feedbackData["feedback-type"];
    const comment = feedbackData.comment || "No comment provided";
    const url = feedbackData.url;

    const emoji = feedbackType === "positive" ? "👍" : "👎";
    const slackMessage = {
      text: `📨 New feedback received`,
      attachments: [
        {
          fields: [
            {
              title: "Feedback Type",
              value: feedbackType === "positive" ? "Positive" : "Negative",
              short: true,
            },
            {
              title: "Page URL",
              value: url,
              short: false,
            },
            {
              title: "Comment",
              value: comment,
              short: false,
            },
          ],
          footer: "OPA Documentation Feedback",
          ts: Math.floor(Date.now() / 1000),
        },
      ],
    };

    const response = await fetch(slackUrl, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(slackMessage),
    });

    if (!response.ok) {
      throw new Error(`Slack webhook failed: ${response.status}`);
    }

    return new Response(JSON.stringify({ success: true }), {
      status: 200,
      headers,
    });
  } catch (error) {
    console.error("Error processing feedback:", error);
    return new Response(JSON.stringify({ error: "Internal server error" }), {
      status: 500,
      headers,
    });
  }
};

