import Link from "@docusaurus/Link";
import Heading from "@theme/Heading";
import ReactMarkdown from "react-markdown";
import styled from "styled-components";

const CardContainer = styled.div`
  border: 1px solid #ddd;
  border-radius: 0.5rem;
  padding: 1rem;
  display: flex;
  flex-direction: column;
  max-width: 25rem;
  height: 100%;
  width: 100%;
`;

const Content = styled.div`
  flex-grow: 1;
`;

const IconWrapper = styled.div`
  height: 3rem;
  display: flex;
  align-items: center;
  justify-content: flex-start;
  margin-bottom: 0.6rem;
`;

const Icon = styled.img`
  max-height: 2.5rem;
  max-width: 3rem;
`;

export default function Card({ item }) {
  return (
    <CardContainer>
      <Content>
        <IconWrapper>
          {item.icon && <Icon src={item.icon} alt={item.title} />}
        </IconWrapper>
        <Heading as="h4">{item.title}</Heading>
        <ReactMarkdown>{item.note}</ReactMarkdown>
      </Content>
      {item.link && (
        <Link className="button button--primary button--sm" to={item.link}>
          {item.link_text}
        </Link>
      )}
    </CardContainer>
  );
}
