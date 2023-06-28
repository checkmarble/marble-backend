import { useCallback, useState } from "react";
import IconButton from "@mui/material/IconButton";
import AppBar from "@mui/material/AppBar";
import Toolbar from "@mui/material/Toolbar";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Button from "@mui/material/Button";
import Drawer from "@mui/material/Drawer";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemIcon from "@mui/material/ListItemIcon";
import ListItemButton from "@mui/material/ListItemButton";
import ListItemText from "@mui/material/ListItemText";
import MenuIcon from "@mui/icons-material/Menu";
import BusinessIcon from "@mui/icons-material/Business";
import PeopleIcon from "@mui/icons-material/People";
import HouseIcon from "@mui/icons-material/House";
import Tooltip from "@mui/material/Tooltip";
import Avatar from "@mui/material/Avatar";
import Menu from "@mui/material/Menu";
import MenuItem from "@mui/material/MenuItem";
import { AuthenticatedUser, PageLink } from "@/models";
import { useNavigate } from "react-router-dom";
import { useAuthenticatedUser } from "@/services";
import services from "@/injectServices";
import { useCredentials } from "@/services";
import { useLoading } from "@/hooks/Loading";
import SettingsIcon from "@mui/icons-material/Settings";

interface Page {
  title: string;
  link: string;
  icon: JSX.Element;
}

const APP_BAR_PAGES: ReadonlyArray<Page> = [
  {
    title: "Home",
    link: PageLink.Home,
    icon: <HouseIcon />,
  },
  {
    title: "Organizations",
    link: PageLink.Organizations,
    icon: <BusinessIcon />,
  },
  {
    title: "Users",
    link: PageLink.Users,
    icon: <PeopleIcon />,
  },
] as const;

function BackOfficeAppBar() {
  const nagivator = useNavigate();

  const [drawerOpen, setDrawerOpen] = useState<boolean>(false);

  const toggleDrawer = () => setDrawerOpen(!drawerOpen);
  const closeDrawer = () => setDrawerOpen(false);

  const user = useAuthenticatedUser();

  return (
    <AppBar position="static">
      <Toolbar disableGutters>
        {/* Desktop App bar */}
        <Typography
          variant="h6"
          noWrap
          component="a"
          href="/"
          sx={{
            display: { xs: "none", md: "flex" },
            mr: 2,
            ml: 2,
            color: "inherit",
            textDecoration: "none",
          }}
        >
          Marble BackOffice
        </Typography>
        <Box sx={{ display: { xs: "none", md: "flex" }, flexgrow: 1 }}>
          {APP_BAR_PAGES.map((page: Page, index) => (
            <Button
              key={index}
              onClick={() => nagivator(page.link)}
              sx={{
                my: 2,
                color: "white",
                display: "block",
              }}
            >
              {page.title}
            </Button>
          ))}
        </Box>
        {/* spacer for desktop */}
        <Box sx={{ display: { xs: "none", md: "flex" }, flexGrow: 1 }} />

        {/* Mobile App Bar */}
        <Box sx={{ display: { xs: "flex", md: "none" }, flexGrow: 1 }}>
          <IconButton
            size="large"
            aria-label="Open drawer"
            color="inherit"
            onClick={toggleDrawer}
          >
            <MenuIcon />
          </IconButton>
          <Typography
            variant="h6"
            noWrap
            component="a"
            href="/"
            sx={{
              display: { xs: "box", md: "none" },
              mr: 2,
              mt: 1,
              color: "inherit",
              textDecoration: "none",
            }}
          >
            Marble BackOffice
          </Typography>
          <Drawer anchor="left" open={drawerOpen} onClose={closeDrawer}>
            <Box
              sx={{
                width: 250,
              }}
              role="presentation"
              onClick={closeDrawer}
              onKeyDown={closeDrawer}
            >
              <List>
                {APP_BAR_PAGES.map((page: Page, index) => (
                  <ListItem key={index} disablePadding>
                    <ListItemButton onClick={() => nagivator(page.link)}>
                      <ListItemIcon>{page.icon}</ListItemIcon>
                      <ListItemText primary={page.title} />
                    </ListItemButton>
                  </ListItem>
                ))}
              </List>
            </Box>
          </Drawer>
        </Box>
        <Box sx={{ display: "flex", flexGrow: 0, mr: 1 }}>
          <SettingsMenu user={user} />
        </Box>
      </Toolbar>
    </AppBar>
  );
}

interface SettingsMenuProps {
  user: AuthenticatedUser;
}
function SettingsMenu({ user }: SettingsMenuProps) {
  const [anchorElUser, setAnchorElUser] = useState<null | HTMLElement>(null);

  const [pageLoading, pageLoadingDispatcher] = useLoading();

  const { credentials } = useCredentials(
    services().userService,
    pageLoadingDispatcher
  );

  const handleOpenUserMenu = (event: React.MouseEvent<HTMLElement>) => {
    setAnchorElUser(event.currentTarget);
  };

  const handleCloseUserMenu = () => {
    setAnchorElUser(null);
  };

  const handleLogout = useCallback(() => {
    handleCloseUserMenu();
    services().authenticationService.signOut();
  }, []);

  return (
    <>
      <Box>
        <Avatar alt="Remy Sharp" src={user.photoURL || ""} />
      </Box>
      <Box mx={2}>
        <Typography sx={{ fontSize: "0.8em" }}>{user.email}</Typography>
        <Typography sx={{ fontSize: "0.8em" }}>{credentials?.role}</Typography>
      </Box>
      <Box>
        <Tooltip title="Open settings">
          <IconButton onClick={handleOpenUserMenu}>
            <SettingsIcon></SettingsIcon>
          </IconButton>
        </Tooltip>
      </Box>

      <Menu
        sx={{ mt: "45px" }}
        id="menu-appbar"
        anchorEl={anchorElUser}
        anchorOrigin={{
          vertical: "top",
          horizontal: "right",
        }}
        keepMounted
        transformOrigin={{
          vertical: "top",
          horizontal: "right",
        }}
        open={Boolean(anchorElUser)}
        onClose={handleCloseUserMenu}
      >
        <MenuItem onClick={handleLogout}>
          <Typography textAlign="center">Logout</Typography>
        </MenuItem>
      </Menu>
    </>
  );
}

export default BackOfficeAppBar;
