@rem 安装netfs服务

@echo install netfs service
sc create httpfs start= auto binPath= ^
""E:\workspace\998-source\gowork\bin\netfs.exe" ^
-http-addr=:8090 ^
-ftp-addr=:8090 ^
-root=t:\ ^
-log.dir="t:\""

@pause
