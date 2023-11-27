import { useEffect, useState } from "react"
import { WoxPluginHelper } from "../../utils/WoxPluginHelper.ts"
import { CircularProgress } from "@mui/material"
import styled from "styled-components"
import { StorePluginManifest } from "../../entity/Plugin.typing"
import "react-image-gallery/styles/css/image-gallery.css"
import WoxPluginList from "./WoxPluginList.tsx"

export default () => {
  const [loading, setLoading] = useState(true)
  const [plugins, setPlugins] = useState<StorePluginManifest[]>([])

  const loadStorePlugins = () => {
    WoxPluginHelper.getInstance()
      .loadStorePlugins()
      .then(_ => {
        setPlugins(WoxPluginHelper.getInstance().getStorePlugins())
        setLoading(false)
      })
  }

  useEffect(() => {
    loadStorePlugins()
  }, [])

  return (
    <Style>
      {loading && <CircularProgress />}
      {!loading && (
        <WoxPluginList
          plugins={plugins}
          type={"store"}
          refreshCallback={() => {
            loadStorePlugins()
          }}
        />
      )}
    </Style>
  )
}

const Style = styled.div`
  .plugin-list-container {
    height: 100%;
    border-right: 1px solid #23272d;
  }

  .plugin-description {
    color: #787b8b;
    display: inline-block;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    width: calc(50vw - 280px);
  }

  .plugin-detail-container {
    width: 100%;
  }

  .plugin-detail-summary {
    padding: 15px;
  }

  .image-gallery-content .image-gallery-slide .image-gallery-image {
    max-height: 350px;
  }
`
