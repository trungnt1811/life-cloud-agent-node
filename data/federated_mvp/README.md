# Dữ liệu MVP Federated Query, phiên bản 2

Bộ dữ liệu này chuẩn bị đầu vào cho ba node bệnh viện giả lập của Life Cloud.
Đây là mô phỏng có truy nguồn từ dữ liệu nghiên cứu, không phải bệnh án thu thập
tại ba bệnh viện Việt Nam. Các file nằm trên cùng máy để phát triển; chưa triển khai
dịch vụ truy vấn phân tán hay database bệnh viện.

## Dùng file nào

| Mục đích | Đường dẫn |
| --- | --- |
| Dữ liệu bệnh viện A | `nodes/VN_A/raw/` |
| Dữ liệu bệnh viện B | `nodes/VN_B/raw/` |
| Dữ liệu bệnh viện C | `nodes/VN_C/raw/` |
| Quy tắc ánh xạ của từng bệnh viện | `nodes/<site>/mapping.json` |
| Một CSV tổng hợp để xem dữ liệu thô | `inspection/all_sites_raw.csv` |
| Ví dụ các ca kiểm thử trong dữ liệu thô | `inspection/example_cases_raw.csv` |
| Dữ liệu chuẩn trước khi thêm lỗi | `benchmark/canonical_measurements.csv` |
| Đáp án cho từng dòng dữ liệu đầu vào | `benchmark/expected_raw_outcomes.csv` |
| Các phép đo kỳ vọng sau loại trùng và chọn phiên bản | `benchmark/expected_measurements.csv` |
| Đáp án số đếm cohort | `benchmark/expected_queries.csv` |
| Định nghĩa chính xác truy vấn | `benchmark/query_definition.json` |
| Nhật ký lỗi được thêm vào | `benchmark/injected_faults.csv` |
| Báo cáo kiểm tra | `verification_report.json` |

Khi dựng node, chỉ cấp cho node đó `nodes/<site>/`. Thư mục `benchmark/` thuộc
bộ kiểm thử; `inspection/` dùng xem dữ liệu ngoại tuyến. Không cấp hai thư mục này
cho pipeline làm sạch hoặc bộ điều phối trung tâm. Nhãn lỗi và giá trị chuẩn
không có trong các file raw của node.

CSV tổng hợp có cùng cột chứa dữ liệu thô để tiện kiểm tra, nhưng vẫn giữ
`test_code`, `value_raw`, `unit_raw`, định dạng ngày và mã bệnh nhân của từng nơi.
Đầu vào chính để chứng minh adapter khác nhau là các file riêng trong `nodes/`.

## Quy mô và nguồn

- 13.031 dòng nguồn HPLC được dùng toàn bộ, không còn lấy mẫu 5.000 dòng.
- 36 hồ sơ hoàn toàn giả lập: 12 kịch bản cố định, lặp lại ở ba node.
- Tổng cộng 13.067 thực thể bệnh nhân trong demo và 119.259 dòng xét nghiệm thô.
- 117.217 dòng được chấp nhận trước chọn phiên bản; 1.602 dòng xuất lại;
  440 dòng cần xử lý riêng; 27 dòng phiên bản cũ bị thay thế.
- Kết quả cuối để đối chiếu: 117.190 phép đo.

Nguồn: [HPLC-based thalassemia screening data trên Kaggle](https://www.kaggle.com/datasets/abhraghoshcmc/hplc-based-thalassemia-screening-data).
File gốc: `data/research_sources/kaggle_hplc_screening/HPLC data.csv` tính từ thư mục dự án.
SHA-256: `51a5729c9864a5292c134fb911e199d628fb4812e19367c8cc503363b009daee`.

Mỗi dòng nguồn được gán một thực thể riêng cho benchmark. Không có bằng chứng
rằng mỗi dòng nguồn tương ứng một người duy nhất ngoài thực tế. Tuổi và nhãn
chẩn đoán được giữ theo nguồn; không suy ra ngày sinh từ tuổi. Mã bệnh nhân,
bệnh viện, ngày xét nghiệm, lượt khám, mẫu xét nghiệm và lịch sử sửa kết quả đều
được mô phỏng. Phân bổ node có khác biệt theo nhãn nguồn nhằm thử tính không
đồng nhất, không đại diện phân bố bệnh thực tế tại Việt Nam hoặc ASEAN.

Các file alpha có chồng lặp chỉ số và khác nhãn; Bangladesh là cohort điều trị;
MRI là nhánh theo dõi ứ sắt; MIMIC không chuyên về thalassemia. Những nguồn này
vẫn nằm nguyên trong `data/research_sources/`, nhưng chưa đưa vào cohort CBC/HPLC.

## Ba kiểu dữ liệu bệnh viện

| Node | Hồ sơ | Cấu trúc raw | Khác biệt cần xử lý |
| --- | ---: | --- | --- |
| VN_A | 6.213 | CSV rộng, một dòng cho một panel | `HGB`, `MRN`, ngày ISO, Hb `g/dL` |
| VN_B | 2.959 | CSV dài, một dòng cho một xét nghiệm | Mã xét nghiệm `0301`, `H02`; Hb `g/L`; HbA2 dạng fraction; ngày `YYYYMMDD` |
| VN_C | 3.895 | CSV xuất từ bảng nhập tay, dấu `;`, UTF-8 BOM | Tên cột tiếng Việt không dấu; số thập phân dấu phẩy; ngày `DD/MM/YYYY`; `g / dL` |

Node C mô phỏng **file CSV xuất từ bảng tính nhập tay**, không cung cấp workbook
`.xlsx`. Tất cả mã định danh phải đọc dưới dạng chuỗi để giữ số 0 đầu. Hai node
có thể cùng mã `0000001`; khóa bệnh nhân trong demo luôn bao gồm node.

`patient_registry.csv` là sổ bệnh nhân cục bộ. Mã lượt khám và mã mẫu ổn định
giữa các xét nghiệm thuộc cùng panel. Định dạng ngày được khai báo theo nguồn
trong cột metadata, không suy luận từ quốc gia. Hàng thiếu khai báo định dạng
và có ngày mơ hồ được đưa vào nhóm cần xử lý riêng.

## Kịch bản và cách xử lý kỳ vọng

| Kịch bản | Kết quả kỳ vọng |
| --- | --- |
| CBC/HPLC đủ dữ liệu | Ghép đúng bệnh nhân, mẫu và lượt khám |
| Chỉ số ngoài bộ điều kiện demo | CBC vẫn hợp lệ, không được đếm vào nhóm khớp điều kiện |
| Thiếu MCH | Không coi là 0; panel không đủ CBC |
| Xuất lại cùng kết quả ở lô thứ hai | Không tạo thêm phép đo hoặc bệnh nhân |
| MCV phiên bản 1 là 70, phiên bản 2 là 85 | Chọn phiên bản 2, không còn khớp điều kiện MCV của demo |
| Cùng người, mẫu tháng sau có MCV 85 | Giữ cả hai mẫu; query chọn mẫu mới nhất |
| Thiếu mã bệnh nhân | Không tự đoán danh tính từ vị trí dòng hoặc ID kết quả |
| Mã lạ, thiếu đơn vị, giá trị âm | Ghi rõ lý do cần xử lý, không tự bù giá trị |
| Ngày `03/04/2024` không biết định dạng | Không tự chọn ngày 3/4 hay 4/3 |
| HbF `<0.1` | Giữ toán tử `<`; đây vẫn là kết quả HPLC có ghi nhận |
| Thiếu HbA2 | Có CBC, chưa đủ panel HPLC trong nguồn đang kết nối |
| HbA2 2.625%, HbF 0.0011% | Đổi fraction và dấu phẩy mà không làm tròn mất thông tin |

Các khác biệt hợp lệ về mã, đơn vị và biểu diễn không phải lỗi dữ liệu. Ví dụ,
Hb `108 g/L` và `10,8 g/dL` đều có thể chuẩn hóa thành `10.8 g/dL`.
Tần suất thêm lỗi khác nhau theo node, được lưu riêng ở `benchmark/*_simulation.json`.
Các tần suất này là tham số demo, chưa được hiệu chỉnh từ khảo sát bệnh viện.

## Truy nguồn

Các cột `source_dataset`, `source_file`, `source_row_number`, `source_record_id`
được giữ trong mọi dòng dữ liệu thô. `source_row_number` là số thứ tự dòng dữ liệu
gốc bắt đầu từ 1, **không tính header**. `source_record_id` là cột `Sl No` của nguồn.
CSV tổng hợp còn có `site`, `raw_file`, `raw_row_number`, `raw_column` để quay lại
đúng ô trong bản xuất bệnh viện; `raw_row_number` cũng không tính header.

File đáp án lưu thêm `source_column` để kiểm tra từng giá trị với cột nguồn.
`synthetic_scenarios` là nhãn nguồn dành riêng cho các ca dựng chủ động; không
diễn giải chúng như dữ liệu của nghiên cứu Kaggle.

## Giới hạn của dữ liệu chuẩn

"Chuẩn" ở đây nghĩa là đúng so với dữ liệu nguồn và phép biến đổi đã quy định,
không phải kết quả lâm sàng đã được xác minh. Đơn vị nguồn theo từ điển giả định
đã ghi trong mapping. `source_profile.csv` cho thấy cả các giá trị bất thường
có sẵn, ví dụ Hb tối đa 41.1 và MCHC tối đa 316 theo đơn vị giả định. Không tự
sửa các giá trị này thành số có vẻ hợp lý hoặc gộp chúng vào lỗi được thêm vào.

Các mã `HB`, `MCV`, `HBA2` là mã benchmark cục bộ. Chưa gán mã LOINC/OMOP và
chưa tuyên bố tuân thủ OMOP CDM. Dữ liệu nguồn không cung cấp kết quả gene đủ
để làm nhãn xác nhận mang gen. Có thể dùng bộ này để kiểm thử adapter và thiết
kế CDM trước khi chốt triển khai chuẩn đầy đủ.

## Đáp án query minh họa

Khoảng thời gian: 01/01/2024 đến 31/12/2024. Chọn mẫu mới nhất có phép đo được
chấp nhận trong khoảng này, trước khi kiểm tra đủ CBC; không quay về một mẫu
cũ chỉ vì mẫu mới thiếu CBC. CBC phải có `HB`, `MCV`, `MCH`, `RBC` với giá trị
chính xác, không phải giới hạn `<` hoặc `>`. Điều kiện demo là `MCV < 80` và
`MCH < 27`. HPLC đủ khi cùng mẫu có `HBA0`, `HBA2`, `HBF`, kể cả kết quả có toán tử.
Đây là tham số kiểm thử kỹ thuật, không phải quy tắc chẩn đoán được thẩm định.

| Node | Hồ sơ | CBC đủ | Khớp điều kiện demo | Trong số đó có đủ HPLC |
| --- | ---: | ---: | ---: | ---: |
| VN_A | 6.213 | 6.076 | 1.090 | 1.089 |
| VN_B | 2.959 | 2.913 | 813 | 812 |
| VN_C | 3.895 | 3.701 | 805 | 804 |

Riêng 12 ca dựng chủ động ở mỗi node phải cho đúng: **12 hồ sơ, 8 CBC đủ,
5 hồ sơ khớp điều kiện, 4 trong số đó có đủ HPLC**. Đáp án này được kiểm tra
bằng giá trị đã tính tay trong test và một triển khai SQL độc lập.
Chưa áp dụng chính sách ẩn nhóm nhỏ trong đáp án kỹ thuật; phần đó thuộc node service sau này.

## Tạo lại và kiểm tra

Chạy từ thư mục dự án:

```bash
python3 scripts/build_federated_demo_data.py
python3 scripts/verify_federated_demo_data.py
python3 -m unittest discover -s tests -v
```

Bộ sinh dùng seed cố định `20260910`; tạo lại cho cùng nội dung dữ liệu. Dữ liệu
nghiên cứu và bộ messy cũ không bị ghi đè. Bộ kiểm tra đọc raw và mapping để
chuẩn hóa đối chiếu, không đọc đáp án để sửa dữ liệu. Test còn kiểm tra hoạt động
khi thư mục đáp án bị ẩn và phát hiện một giá trị raw bị sửa ngoài ý muốn.
