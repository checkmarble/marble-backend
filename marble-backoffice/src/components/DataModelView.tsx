import { type LoadingDispatcher } from "@/hooks/Loading";
import services from "@/injectServices";
import { useDataModel, useDeleteDataModel } from "@/services";
import DataModelAPIDoc from "@/components/DataModelAPIDoc";
import Button from "@mui/material/Button";
import Box from "@mui/material/Box";

export function DataModelView({
  pageLoadingDispatcher,
  organizationId,
}: {
  pageLoadingDispatcher: LoadingDispatcher;
  organizationId: string;
}) {
  const { dataModel, refreshDataModel } = useDataModel(
    services().organizationService,
    pageLoadingDispatcher,
    organizationId
  );

  const { cleanDataModel } = useDeleteDataModel({
    service: services().organizationService,
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

  const handleCleanDataModel = () => {
    void cleanDataModel();
  };

  return (
    <>
      <Box sx={{ my: 2 }}>
        <Button variant="contained" onClick={handleCleanDataModel}>
          Clean Data Model and Organization Schema
        </Button>
      </Box>

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
