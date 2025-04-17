import Link from "@docusaurus/Link";
import Heading from "@theme/Heading";
import ReactMarkdown from "react-markdown";

export default function Card({ item }) {
  return (
    <div
      style={{
        border: "1px solid #ddd",
        borderRadius: 8,
        padding: 16,
        marginBottom: 16,
        maxWidth: 400,
      }}
    >
      {item.icon && (
        <img
          src={item.icon}
          alt={item.title}
          style={{ maxWidth: 40, marginBottom: 10 }}
        />
      )}
      <Heading as="h4">{item.title}</Heading>
      <ReactMarkdown>
        {item.note}
      </ReactMarkdown>
      {item.links && (
        <ul>
          {item.links.map((link, idx) => (
            <li key={idx}>
              <a href={link.url} target="_blank" rel="noopener noreferrer">
                {link.text}
              </a>
            </li>
          ))}
        </ul>
      )}
      {item.link && (
        <Link className="button button--primary button--sm" to={item.link}>
          {item.link_text}
        </Link>
      )}
    </div>
  );
}
