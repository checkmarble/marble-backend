import { DataModel } from "@/models";
import { Divider, Paper, TextareaAutosize, Typography } from "@mui/material";
import { Fragment } from "react";
import ReactJson from "react-json-view";

interface DataModelAPIDocProps {
  dataModel: DataModel | null;
}

export default function DataModelAPIDoc(props: DataModelAPIDocProps) {
  const dataModel = props.dataModel;

  if (dataModel === null) {
    return <>null dataModel</>;
  }

  const apiModel = dataModelToObjects(dataModel);

  return (
    <Fragment>
      <Typography variant="h6">
        Custom objects to use in the Ingestion and Decision API
      </Typography>
      {apiModel.map((obj) => (
        <Paper sx={{ p: 2, m: 2 }} variant="outlined" key={obj.name}>
          <Typography sx={{ fontFamily: "monospace" }}>
            {obj.name} (POST /ingestion/{obj.name})
          </Typography>
          <Divider sx={{ my: 2 }} />
          <ReactJson src={obj.example} style={{ fontSize: ".7em" }}></ReactJson>
          <Divider sx={{ my: 2 }} />

          <TextareaAutosize
            value={obj.displayableJSON}
            style={{ width: "100%" }}
          />
        </Paper>
      ))}
    </Fragment>
  );
}

interface DataModelObject {
  name: string;
  example: { [key: string]: boolean | number | string };
  displayableJSON: string;
}

function dataModelToObjects(dataModel: DataModel) {
  const customObjects: DataModelObject[] = [];

  for (const [tableName, table] of Object.entries(dataModel.tables)) {
    const validJSON: { [key: string]: boolean | number | string } = {};
    let rawJSONwithComments = "{";

    for (const [fieldName, field] of Object.entries(table.fields)) {
      switch (field.dataType) {
        case "Bool":
          validJSON[fieldName] = false;
          break;
        case "Float":
          validJSON[fieldName] = 1.99;
          break;
        case "Int":
          validJSON[fieldName] = 1;
          break;
        case "String":
          validJSON[fieldName] = "string";
          break;
        case "Timestamp":
          validJSON[fieldName] = Date.now();
          break;
        case "unknown":
          validJSON[fieldName] = "type unknown";
          break;
      }

      rawJSONwithComments += `\n\t"${fieldName}": ...\t//`;
      rawJSONwithComments += field.nullable ? "optional" : "required";
      rawJSONwithComments += `, ${field.dataType}`;
    }

    rawJSONwithComments += "\n}";

    customObjects.push({
      name: tableName,
      example: validJSON,
      displayableJSON: rawJSONwithComments,
    });
  }

  return customObjects;
}
