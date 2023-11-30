import { Box, Checkbox, FormControl, FormControlLabel, MenuItem, Select, TextField } from "@mui/material"
import { PluginSettingDefinitionItem } from "../../entity/Plugin.typing"
import styled from "styled-components"
import { UpdateSetting } from "../../entity/Setting.typings"
import { WoxSettingHelper } from "../../utils/WoxSettingHelper.ts"

export default (props: { pluginId: string; settingDefinitions: PluginSettingDefinitionItem[] }) => {
  const { pluginId, settingDefinitions } = props

  const executeSettingUpdate = async (updateSetting: UpdateSetting) => {
    await WoxSettingHelper.getInstance().updateSetting(updateSetting)
  }

  const stringToBoolean = (value: string | null | undefined) => {
    if (value === null || value === undefined) {
      return false
    }
    return value.toLowerCase() === "true"
  }

  const getSettingItem = (settingDefinition: PluginSettingDefinitionItem) => {
    switch (settingDefinition.Type) {
      case "checkbox":
        return (
          <FormControlLabel
            control={
              <Checkbox
                defaultChecked={stringToBoolean(settingDefinition.Value?.DefaultValue)}
                onChange={async event => {
                  await executeSettingUpdate({ Key: settingDefinition.Value?.Key || "", Value: event.target.checked + "" })
                }}
              />
            }
            label={settingDefinition.Value?.Label}
          />
        )
      case "textbox":
        return (
          <Box>
            {settingDefinition.Value?.Label && <span style={{ paddingRight: "5px", display: "inline-block" }}>{settingDefinition.Value?.Label}</span>}
            <TextField hiddenLabel defaultValue={settingDefinition.Value?.DefaultValue} size="small" />
            {settingDefinition.Value?.Suffix && <span style={{ paddingLeft: "5px", display: "inline-block" }}>{settingDefinition.Value?.Suffix}</span>}
          </Box>
        )
      case "newline":
        return <br />
      case "select":
        return (
          <FormControl sx={{ m: 1, minWidth: 220 }} size="small">
            <Select
              defaultValue={settingDefinition.Value?.DefaultValue}
              onChange={async event => {
                await executeSettingUpdate({ Key: settingDefinition.Value?.Key || "", Value: event.target.value })
              }}
            >
              {settingDefinition.Value?.Options?.map((option, index) => {
                return (
                  <MenuItem key={`option-${index}`} value={option.Value}>
                    {option.Label}
                  </MenuItem>
                )
              })}
            </Select>
          </FormControl>
        )
      default:
        return <div></div>
    }
  }

  return (
    <Style id={pluginId}>
      {settingDefinitions.map((settingDefinition, index) => {
        return (
          <div key={`setting-item-${index}`}>
            <FormControl sx={{ m: 1 }} size="small">
              {getSettingItem(settingDefinition)}
            </FormControl>
          </div>
        )
      })}
    </Style>
  )
}

const Style = styled.div``
