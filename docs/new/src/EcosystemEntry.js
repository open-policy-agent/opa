import Heading from "@theme/Heading";
import Layout from "@theme/Layout";
import React from "react";
import ReactMarkdown from "react-markdown";

const EcosystemEntry = (props) => {
  const data = props.route.customData;

  const {
    id,
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
  } = data;

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

        {/* Labels */}
        {labels && (
          <div style={{ margin: "1rem 0" }}>
            <strong>Category:</strong> {labels.category} <br />
            <strong>Layer:</strong> {labels.layer}
          </div>
        )}

        {/* Inventors */}
        {inventors?.length > 0 && (
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
          <div style={{ marginBottom: "1rem" }}>
            <strong>Videos:</strong>
            <ul>
              {videos.map((video, idx) => (
                <li key={`video-${idx}`}>
                  <a href={video.link} target="_blank" rel="noopener noreferrer">
                    {video.title}
                  </a>
                  {video.venue && <span style={{ marginLeft: 8 }}>({video.venue})</span>}
                  {video.speakers?.length > 0 && (
                    <div style={{ fontSize: "0.9em", color: "#555" }}>
                      {video.speakers.map((s) => `${s.name} (${s.organization})`).join(", ")}
                    </div>
                  )}
                </li>
              ))}
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

        {/* Content (Markdown) */}
        {content && (
          <div style={{ marginTop: "2rem" }}>
            <strong>Description:</strong>
            <div style={{ marginTop: "0.5rem" }}>
              <ReactMarkdown>
                {content}
              </ReactMarkdown>
            </div>
          </div>
        )}
      </div>
    </Layout>
  );
};

export default EcosystemEntry;
