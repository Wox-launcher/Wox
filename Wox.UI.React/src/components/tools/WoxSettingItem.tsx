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
            sx={{ margin: "5px 0", paddingRight: "5px" }}
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
          <FormControl sx={{ m: 1, padding: 0, margin: "5px 0" }}>
            <Box sx={{ padding: 0 }}>
              {settingDefinition.Value?.Label && <span style={{ paddingLeft: "5px", paddingRight: "5px", lineHeight: "40px", display: "inline-block" }}>{settingDefinition.Value?.Label}</span>}
              <TextField sx={{ width: "100px" }} hiddenLabel defaultValue={settingDefinition.Value?.DefaultValue} size="small" />
              {settingDefinition.Value?.Suffix && <span style={{ paddingLeft: "5px", paddingRight: "5px", lineHeight: "40px", display: "inline-block" }}>{settingDefinition.Value?.Suffix}</span>}
            </Box>
          </FormControl>
        )
      case "newline":
        return <Box />
      case "select":
        return (
          <FormControl sx={{ m: 1, margin: "5px 0", padding: 0 }} size="small">
            <Box sx={{ padding: 0 }}>
              {settingDefinition.Value?.Label && <span style={{ paddingLeft: "5px", paddingRight: "5px", lineHeight: "40px", display: "inline-block" }}>{settingDefinition.Value?.Label}</span>}
              <Select
                sx={{ width: "100px" }}
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
              {settingDefinition.Value?.Suffix && <span style={{ paddingLeft: "5px", paddingRight: "5px", lineHeight: "40px", display: "inline-block" }}>{settingDefinition.Value?.Suffix}</span>}
            </Box>
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
          <span className={"setting-item"} key={`setting-item-${index}`}>
            {getSettingItem(settingDefinition)}
          </span>
        )
      })}
    </Style>
  )
}

const Style = styled.div`
  padding: 10px;
  .MuiCheckbox-root,
  .Mui-disabled {
    color: #1976d2;
  }

  .MuiOutlinedInput-notchedOutline,
  .MuiOutlinedInput-notchedOutline:hover {
    border: 1px solid white;
  }

  .MuiInputLabel-root,
  .MuiSelect-select,
  .MuiSelect-icon,
  .MuiInputBase-input {
    color: white;
  }
`
