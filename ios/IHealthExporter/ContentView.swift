import SwiftUI

struct ContentView: View {
    @AppStorage("serverURL") private var serverURL = "http://192.168.3.10:8124"
    @AppStorage("serverToken") private var serverToken = ""
    @AppStorage("lastSuccessfulSync") private var lastSync = 0.0
    @StateObject private var exporter = HealthExporter()

    var body: some View {
        NavigationStack {
            Form {
                Section("Сервер на NAS") {
                    TextField("http://IP:PORT", text: $serverURL).textInputAutocapitalization(.never).autocorrectionDisabled().keyboardType(.URL)
                    SecureField("Bearer-токен", text: $serverToken).textInputAutocapitalization(.never).autocorrectionDisabled()
                }
                Section("Синхронизация") {
                    if exporter.isRunning {
                        ProgressView(value: exporter.progress)
                        Text("sent: \(exporter.sentRecords)   skip: \(exporter.skippedTypes)   error: \(exporter.failedTypes)")
                            .monospacedDigit()
                    }
                    Text(exporter.status).font(.footnote).foregroundStyle(.secondary)
                    if lastSync > 0 { Text("Последняя успешная: \(Date(timeIntervalSince1970: lastSync).formatted())").font(.footnote) }
                    Button(exporter.isRunning ? "Выполняется…" : "Синхронизировать") {
                        Task { await exporter.synchronize(baseURL: serverURL, token: serverToken) }
                    }.disabled(exporter.isRunning || serverURL.isEmpty || serverToken.isEmpty)
                }
                Section { Text("Первый запуск выгружает всю доступную историю. Следующие передают только изменения и удаления.") }
            }
            .navigationTitle("iHealth Exporter")
        }
    }
}
