import React from "react";
import styled from "styled-components";

const Container = styled.div`
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;

  @media (max-width: 1200px) {
    flex-direction: column;
  }
`;

const Child = styled.div`
  margin-top: 1rem;
  width: 50%;

  &:first-child {
    padding-right: 0.25rem;
  }

  &:last-child {
    padding-left: 0.25rem;
  }

  @media (max-width: 1200px) {
    width: 100%;
    padding: 0;
  }
`;

export default function SideBySideContainer({ children }) {
  return (
    <Container>
      {React.Children.map(children, (child, index) => <Child>{child}</Child>)}
    </Container>
  );
}
