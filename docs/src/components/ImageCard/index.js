import Link from "@docusaurus/Link";
import Heading from "@theme/Heading";

export default function ImageCard({ item }) {
  return (
    <div
      style={{
        border: "1px solid #ddd",
        borderRadius: 8,
        padding: 16,
        marginBottom: 16,
        maxWidth: 400,
        textAlign: "center",
      }}
    >
      {item.image && (
        <img
          src={item.image}
          alt={item.title}
          style={{ width: "100%", height: "auto", borderRadius: 8, marginBottom: 10 }}
        />
      )}
      <Heading as="h4" style={{ marginTop: 10 }}>
        {item.title}
      </Heading>
      <p>{item.note}</p>
      {item.links && (
        <ul style={{ listStyleType: "none", padding: 0 }}>
          {item.links.map((link, idx) => (
            <li key={idx} style={{ marginBottom: 5 }}>
              <a href={link.url} target="_blank" rel="noopener noreferrer">
                {link.text}
              </a>
            </li>
          ))}
        </ul>
      )}
      {item.link && (
        <Link className="button button--primary button--sm" to={item.link} style={{ marginTop: 10 }}>
          {item.link_text}
        </Link>
      )}
    </div>
  );
}
