import Heading from "@theme/Heading";
import Layout from "@theme/Layout";
import React from "react";
import ReactMarkdown from "react-markdown";

import entries from "@generated/ecosystem-data/default/entries.json";

const EcosystemEntry = (props) => {
  const { id } = props.route.customData;
  const page = entries[id];

  const {
    title,
    subtitle,
    labels,
    inventors,
    blogs,
    code,
    videos,
    tutorials,
    content,
    logo,
  } = page;

  return (
    <Layout title={title}>
      <div className="container margin-vert--lg">
        <div style={{ display: "flex", alignItems: "center", marginBottom: "1rem" }}>
          {/* Logo */}
          <img
            src={logo}
            alt={`${title} Logo`}
            style={{ maxWidth: "150px", height: "auto", marginRight: "1rem" }}
          />

          {/* Title */}
          <Heading as="h1" style={{ margin: 0 }}>
            {title}
          </Heading>
        </div>

        {/* Subtitle */}
        {subtitle && <p style={{ fontSize: "1.2rem", color: "#555" }}>{subtitle}</p>}

        {/* Content (Markdown) */}
        {content && (
          <div style={{ marginTop: "2rem" }}>
            <div style={{ marginTop: "0.5rem" }}>
              <ReactMarkdown>
                {content}
              </ReactMarkdown>
            </div>
          </div>
        )}

        {/* Inventors */}
        {/* Disabled until we have inventor pages */}
        {false && inventors?.length > 0 && (
          <div style={{ marginBottom: "1rem" }}>
            <strong>Inventors:</strong> {inventors.join(", ")}
          </div>
        )}

        {/* Blogs */}
        {blogs?.length > 0 && (
          <div style={{ marginBottom: "1rem" }}>
            <strong>Blogs:</strong>
            <ul>
              {blogs.map((url, idx) => (
                <li key={`blog-${idx}`}>
                  <a href={url} target="_blank" rel="noopener noreferrer">
                    {url}
                  </a>
                </li>
              ))}
            </ul>
          </div>
        )}

        {/* Code */}
        {code?.length > 0 && (
          <div style={{ marginBottom: "1rem" }}>
            <strong>Code:</strong>
            <ul>
              {code.map((url, idx) => (
                <li key={`code-${idx}`}>
                  <a href={url} target="_blank" rel="noopener noreferrer">
                    {url}
                  </a>
                </li>
              ))}
            </ul>
          </div>
        )}

        {/* Videos */}
        {videos?.length > 0 && (
          <div>
            <strong>Videos:</strong>
            <ul>
              {videos.map((video, idx) => {
                // If the video is a simple string URL
                if (typeof video === "string") {
                  return (
                    <li key={`video-${idx}`}>
                      <a href={video} target="_blank" rel="noopener noreferrer">
                        {video}
                      </a>
                    </li>
                  );
                }

                // Structured video object
                return (
                  <li key={`video-${idx}`}>
                    <a href={video.link} target="_blank" rel="noopener noreferrer">
                      {video.title}
                      {video.venue && ` - ${video.venue}`}
                    </a>
                    {Array.isArray(video.speakers) && (
                      <ul>
                        {video.speakers.map((speaker, sIdx) => {
                          if (typeof speaker === "string") {
                            return <li key={`speaker-${sIdx}`}>{speaker}</li>;
                          }

                          return (
                            <li key={`speaker-${sIdx}`}>
                              {speaker.name} - {speaker.organization}
                            </li>
                          );
                        })}
                      </ul>
                    )}
                  </li>
                );
              })}
            </ul>
          </div>
        )}

        {/* Tutorials */}
        {tutorials?.length > 0 && (
          <div style={{ marginBottom: "1rem" }}>
            <strong>Tutorials:</strong>
            <ul>
              {tutorials.map((url, idx) => (
                <li key={`tutorial-${idx}`}>
                  <a href={url} target="_blank" rel="noopener noreferrer">
                    {url}
                  </a>
                </li>
              ))}
            </ul>
          </div>
        )}

        {/* Labels */}
        {labels && (
          <div style={{ margin: "1rem 0" }}>
            <strong>Category:</strong> {labels.category} <br />
            <strong>Layer:</strong> {labels.layer}
          </div>
        )}
      </div>
    </Layout>
  );
};

export default EcosystemEntry;
