if GOVIMPluginStatus() == "initcomplete"
  call GOVIMFtplugin(expand('<amatch>'))
endif
