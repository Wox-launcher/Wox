import React, { useState } from "react"
import { Tab, Tabs } from "@mui/material"
import PhoneIcon from "@mui/icons-material/Phone"
import FavoriteIcon from "@mui/icons-material/Favorite"
import PersonPinIcon from "@mui/icons-material/PersonPin"
import styled from "styled-components"

export default () => {
  const [tabIndex, setTabIndex] = useState(0)

  const handleTabChange = (_event: React.SyntheticEvent, newValue: number) => {
    setTabIndex(newValue)
  }

  return <Style>
    <Tabs value={tabIndex} onChange={handleTabChange}>
      <Tab icon={<PhoneIcon />} label="RECENTS" />
      <Tab icon={<FavoriteIcon />} label="FAVORITES" />
      <Tab icon={<PersonPinIcon />} label="NEARBY" />
    </Tabs>
  </Style>
}

const Style = styled.div`
  display: flex;
  justify-content: center;
`