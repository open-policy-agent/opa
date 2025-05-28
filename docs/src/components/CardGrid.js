import React from "react";
import styled from "styled-components";

const FlexGrid = styled.div`
  display: flex;
  flex-wrap: wrap;
  justify-content: ${({ justifyContent }) => justifyContent || "center"};
  gap: 1.25rem;
  margin-top: 2rem;
  margin-bottom: 2rem;
  margin-left: auto;
  margin-right: auto;
`;

const GridItem = styled.div`
  flex-grow: 1;
  flex-shrink: 0;
  flex-basis: 20rem;
  max-width: 25rem;
  display: flex;
`;

const CardGrid = ({ justifyContent = "center", children }) => {
  return (
    <FlexGrid justifyContent={justifyContent}>
      {React.Children.map(children, (child, idx) => <GridItem key={idx}>{child}</GridItem>)}
    </FlexGrid>
  );
};

export default CardGrid;
