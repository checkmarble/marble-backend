import { useLoading } from "@/hooks/Loading";
import services from "@/injectServices";
import { PageLink } from "@/models";
import { useOrganization } from "@/services";
import { CircularProgress, IconButton, Typography } from "@mui/material";
import {
  DefaultComponentProps,
  OverridableTypeMap,
} from "@mui/material/OverridableComponent";
import { useNavigate } from "react-router-dom";
import NorthEastIcon from "@mui/icons-material/NorthEast";

type InlineOrganizationFromIdProps<M extends OverridableTypeMap> =
  DefaultComponentProps<M> & {
    organizationId: string;
  };

export default function InlineOrganizationFromId<M extends OverridableTypeMap>({
  organizationId,
  ...typographyProps
}: InlineOrganizationFromIdProps<M>) {
  const [loading, loadingDispatcher] = useLoading();

  const { organization } = useOrganization(
    services().organizationService,
    loadingDispatcher,
    organizationId
  );

  const navigate = useNavigate();

  //debugger;

  if (loading) {
    return (
      <>
        <CircularProgress color="secondary" size="1em" />
        <Typography {...typographyProps}>{organizationId}</Typography>
      </>
    );
  }
  return (
    <>
      <Typography {...typographyProps} sx={{ p: 0 }}>
        {organization?.name}
      </Typography>
      <IconButton
        aria-label="details"
        size="large"
        onClick={() => navigate(PageLink.organizationDetails(organizationId))}
        color="secondary"
        sx={{ p: 0 }}
      >
        <NorthEastIcon fontSize="inherit" />
      </IconButton>
    </>
  );
}
