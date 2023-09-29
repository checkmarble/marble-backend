import { type LoadingDispatcher } from "@/hooks/Loading";
import services from "@/injectServices";
import { useDataModel, useEditDataModel } from "@/services";
import DataModelAPIDoc from "@/components/DataModelAPIDoc";
import Button from "@mui/material/Button";
import Stack from "@mui/material/Stack";
import AlertDialog from "./AlertDialog";
import Typography from "@mui/material/Typography";

export function DataModelView({
  pageLoadingDispatcher,
  organizationId,
}: {
  pageLoadingDispatcher: LoadingDispatcher;
  organizationId: string;
}) {
  const service = services().dataModelService;
  const { dataModel, refreshDataModel } = useDataModel(
    service,
    pageLoadingDispatcher,
    organizationId
  );

  const {
    cleanDataModelConfirmed,
    cleanDataModelAlertDialogOpen,
    setCleanDataModelAlertDialogOpen,
    createDemoDataModel,
  } = useEditDataModel({
    service,
    loadingDispatcher: pageLoadingDispatcher,
    organizationId,
    refreshDataModel,
  });

  // const {
  //   dataModelString, setDataModelString, saveDataModel, saveDataModelConfirmed, dataModelError, saveDataModelAlertDialogOpen, setSaveDataModelAlertDialogOpen, canSave,
  // } = useEditDataModel(
  //   services().organizationService,
  //   pageLoadingDispatcher,
  //   organizationId,
  //   dataModel
  // );

  return (
    <>
      {/* Dialog: Replace Data Nodel */}
      <AlertDialog
        title="Confirm client's data deletion"
        open={cleanDataModelAlertDialogOpen}
        handleClose={() => {
          setCleanDataModelAlertDialogOpen(false);
        }}
        handleValidate={cleanDataModelConfirmed}
      >
        <Typography variant="body1">
          Are you sure to remove the Data Model corresponding client's table? This action is destructive:
          all the ingested data of this organization will be erased.
        </Typography>
      </AlertDialog>
      <Stack sx={{ my: 2 }} gap={2}>
        <Button variant="contained" color="warning" onClick={() => setCleanDataModelAlertDialogOpen(true)}>
          Clean Data Model and Organization Schema
        </Button>
        <Button
          variant="contained"
          onClick={createDemoDataModel}
        >
          Create Demo Data Model
        </Button>
      </Stack>

      <DataModelAPIDoc dataModel={dataModel} />
    </>
  );
  // return dataModelString ? (
  //   <>
  //     {/* Dialog: Replace Data Nodel */}
  //     <AlertDialog
  //       title="Confirm organization deletion"
  //       open={saveDataModelAlertDialogOpen}
  //       handleClose={() => {
  //         setSaveDataModelAlertDialogOpen(false);
  //       }}
  //       handleValidate={saveDataModelConfirmed}
  //     >
  //       <Typography variant="body1">
  //         Are you sure to replace the Data Model ? This action is destructive:
  //         all the ingested data of this organization will be erased.
  //       </Typography>
  //     </AlertDialog>
  //     {dataModelString !== null && (
  //       <Box
  //         sx={{
  //           mb: 4,
  //         }}
  //       >
  //         <TextareaAutosize
  //           minRows="5"
  //           value={dataModelString}
  //           style={{ width: "100%" }}
  //           onChange={(e) => setDataModelString(e.target.value)} />
  //         {dataModelError && <Alert severity="error">{dataModelError}</Alert>}
  //         <Button
  //           onClick={saveDataModel}
  //           variant="contained"
  //           startIcon={<DeleteForever />}
  //           color="warning"
  //           disabled={!canSave}
  //         >
  //           Replace Data Model
  //         </Button>

  //         <Divider
  //           sx={{
  //             my: 2,
  //           }}
  //         ></Divider>

  //       </Box>
  //     )}
  //   </>
  // ) : (
  //   false
  // );
}
