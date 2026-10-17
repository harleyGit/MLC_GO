package SQLQueriesPackage

// HGExternalCommentTargetSQL 按BVID主键检查已入库快照，外部评论使用external:bilibili:BV号作为整片身份。
// 两个参数均为完整身份；二进制比较拒绝大小写别名，避免形成不同缓存键。
const HGExternalCommentTargetSQL = `SELECT CONCAT('external:bilibili:',bvid) FROM crawler_video_details
 WHERE bvid=SUBSTRING(?,19) AND BINARY CONCAT('external:bilibili:',bvid)=BINARY ? LIMIT 1`

// HGExternalDanmakuTargetSQL 主键点查后展开最多1000个分P；CID而非页序号作为稳定时间线身份。
// 参数为external:bilibili:BV号:CID，最多64字符；返回video_id及整片评论身份。
const HGExternalDanmakuTargetSQL = `SELECT CONCAT('external:bilibili:',d.bvid,':',p.cid),CONCAT('external:bilibili:',d.bvid)
 FROM crawler_video_details d
 JOIN JSON_TABLE(d.snapshot,'$.parts[*]' COLUMNS(cid VARCHAR(20) PATH '$.cid')) p
 WHERE d.bvid=SUBSTRING_INDEX(SUBSTRING(?,19),':',1)
 AND BINARY CONCAT('external:bilibili:',d.bvid,':',p.cid)=BINARY ? LIMIT 1`
