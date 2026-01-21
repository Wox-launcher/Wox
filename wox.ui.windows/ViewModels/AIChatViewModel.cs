using System;
using System.Collections.ObjectModel;
using System.Text.Json;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Wox.UI.Windows.Models;
using Wox.UI.Windows.Services;

namespace Wox.UI.Windows.ViewModels;

public partial class AIChatViewModel : ObservableObject
{
    private readonly WoxApiService _apiService;

    [ObservableProperty]
    private AIChatData? _currentChat;

    [ObservableProperty]
    private ObservableCollection<AIChatConversation> _conversations = new();

    [ObservableProperty]
    private string _inputText = string.Empty;

    [ObservableProperty]
    private bool _isVisible;

    [ObservableProperty]
    private string _chatTitle = "New Chat";

    [ObservableProperty]
    private AIModel? _currentModel;

    [ObservableProperty]
    private bool _isLoading;

    public AIChatViewModel()
    {
        _apiService = WoxApiService.Instance;
        _apiService.ChatResponseReceived += OnChatResponseReceived;
        _apiService.FocusToChatInputRequested += OnFocusToChatInputRequested;
        _apiService.ReloadChatResourcesRequested += OnReloadChatResourcesRequested;
    }

    public void LoadChatData(string json)
    {
        try
        {
            var chatData = JsonSerializer.Deserialize<AIChatData>(json);
            if (chatData != null)
            {
                CurrentChat = chatData;
                ChatTitle = chatData.Title ?? "Chat";
                CurrentModel = chatData.Model;

                Conversations.Clear();
                if (chatData.Conversations != null)
                {
                    foreach (var conv in chatData.Conversations)
                    {
                        Conversations.Add(conv);
                    }
                }
            }
        }
        catch (Exception ex)
        {
            Logger.Error("Error loading chat data", ex);
        }
    }

    private void OnChatResponseReceived(object? sender, string json)
    {
        System.Windows.Application.Current.Dispatcher.Invoke(() =>
        {
            try
            {
                var chatData = JsonSerializer.Deserialize<AIChatData>(json);
                if (chatData != null)
                {
                    CurrentChat = chatData;
                    ChatTitle = chatData.Title ?? "Chat";

                    // Update conversations
                    Conversations.Clear();
                    if (chatData.Conversations != null)
                    {
                        foreach (var conv in chatData.Conversations)
                        {
                            Conversations.Add(conv);
                        }
                    }
                }
            }
            catch (Exception ex)
            {
                Logger.Error("Error parsing chat response", ex);
            }
        });
    }

    private void OnFocusToChatInputRequested(object? sender, EventArgs e)
    {
        System.Windows.Application.Current.Dispatcher.Invoke(() =>
        {
            IsVisible = true;
            // Focus will be handled by the view
        });
    }

    private void OnReloadChatResourcesRequested(object? sender, string resourceName)
    {
        // Reload AI models or other resources as needed
        Logger.Log($"Reload chat resources requested: {resourceName}");
    }

    [RelayCommand]
    private void Show()
    {
        IsVisible = true;
    }

    [RelayCommand]
    private void Hide()
    {
        IsVisible = false;
    }

    [RelayCommand]
    private void ClearChat()
    {
        Conversations.Clear();
        InputText = string.Empty;
        ChatTitle = "New Chat";
    }
}
