@rem 安装服务

@echo install httpfs service
sc create httpfs start= auto binPath= ^
""E:\workspace\998-source\gowork\bin\httpfs.exe" ^
-addr=:8090 ^
-dir=t:\ ^
-log.dir="t:\""

@pause
