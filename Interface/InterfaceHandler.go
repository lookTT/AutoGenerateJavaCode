package Interface

import "AutoGenerateJavaCode/Model"

var strLimit = `package {{.PackageApi}};

import com.alibaba.fastjson.JSONObject;
import {{.PackageFallback}}.AppOmAppletsFallback;
import com.zt.appoperatemanage.model.applets.*;
import com.zt.common.PageFinder;
import com.zt.common.R;
import org.springframework.cloud.openfeign.FeignClient;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestMethod;

import java.util.List;
import java.util.Map;
`

func DoSomeWork(config *Model.SConfig, curTableName string, sFieldInfos []Model.SFieldInfo, size int)  {
	
}