@echo off
REM Environment Variables
REM LIMA_INSTANCE: Specifies the name of the Lima instance to use. Default is "default".
REM LIMACTL: Specifies the path to the limactl binary. Default is "limactl" in %PATH%.

IF NOT DEFINED LIMACTL (SET LIMACTL=limactl)
IF NOT DEFINED LIMA_INSTANCE (SET LIMA_INSTANCE=default)
%LIMACTL% shell %LIMA_INSTANCE% %*
